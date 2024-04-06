package service

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"vilan/app"
	"vilan/arp"
	"vilan/common"
	"vilan/config"
	"vilan/model"
	"vilan/netty"
	"vilan/netty/codec/format"
	"vilan/netty/transport"
	"vilan/netty/transport/udp"
	"vilan/protocol"
	"vilan/sys/wifi"
)

var gratuitousArp = []byte{
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, /* dest MAC */
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, /* src MAC */
	0x08, 0x06, /* ARP */
	0x00, 0x01, /* ethernet */
	0x08, 0x00, /* IP */
	0x06,       /* hw Size */
	0x04,       /* protocol Size */
	0x00, 0x02, /* ARP reply */
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, /* src MAC */
	0x00, 0x00, 0x00, 0x00, /* src IP */
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, /* target MAC */
	0x00, 0x00, 0x00, 0x00, /* target IP */
}

type RuntimeService struct {
	isInit        bool
	running       bool
	appConfig     *model.AppConfig // 重启后 更新配置
	cancelContext context.Context
	cancelFunc    context.CancelFunc

	crypt common.Crypt // 加密 解密

	peerState     model.PeerState
	serverSock    *model.ServerSockContext
	serverChannel netty.Channel
	linkMode      model.LinkMode // 连接方式
	linkQuality   uint32         // 信号强度
	p2pListener   *net.TCPListener
	externSock    *protocol.Sock
	LocalIp       *protocol.IpNet
	LocalIpStr    string
	stats         *protocol.Statistics
	lastRoute     *model.RouteInfo // 可以考虑保存多个route

	groupPeerCookie uint32    // 上次请求应答的cookie
	groupPeers      *sync.Map //map[uint64]*protocol.PeerInfo

	linkInfos         map[uint64][]*protocol.LinkInfo
	linkInfoReqCookie map[uint64]uint32
	linkMutex         sync.RWMutex

	configSend map[uint64]chan error
}

func NewRuntimeService() *RuntimeService {
	conf := &model.AppConfig{}
	_ = config.CopyAppConfigTo(conf)
	r := &RuntimeService{isInit: false, running: false, appConfig: conf, peerState: model.StateUnInit}
	return r
}

func (r *RuntimeService) Init() (err error) {
	defer func() {
		if e := recover(); e != nil {
			r.SetPeerState(model.StateInitError)
			app.Logger.Error(fmt.Sprintf("运行配置初始化失败:%s", e))
		}
	}()
	r.stats = &protocol.Statistics{}
	r.LocalIpStr = ""
	r.serverSock = &model.ServerSockContext{Heartbeat: r.appConfig.Heartbeat, Offline: r.appConfig.Offline}
	r.groupPeers = &sync.Map{}
	r.linkInfos = make(map[uint64][]*protocol.LinkInfo)
	r.linkInfoReqCookie = make(map[uint64]uint32)
	r.configSend = make(map[uint64]chan error)
	r.SetPeerState(model.StateInitOk)
	return err
}

func (r *RuntimeService) Start() (err error) {
	if r.running {
		return
	}
	r.running = true
	r.cancelContext, r.cancelFunc = context.WithCancel(context.Background()) // 全局取消
	err = r.initCrypt()
	go r.stateCheck()
	return err
}

func (r *RuntimeService) Stop() {
	_ = r.SendUnAuth() //
	r.CleanRoutes()
	time.Sleep(1000 * time.Millisecond)
	r.running = false
	r.cancelFunc() // 取消
	r.SetPeerState(model.StateUnConn)
}

func (r *RuntimeService) Restart() error {
	defer func() {
		recover()
	}()
	app.Logger.Info("服务开始重启...")
	app.P2pService.Stop()
	r.Stop()
	time.Sleep(50 * time.Millisecond)
	if err := app.TunTapService.Stop(); err != nil {
		return err
	}
	// 使用新配置
	_ = config.CopyAppConfigTo(r.appConfig)
	time.Sleep(100 * time.Millisecond)
	if r.appConfig.TapConfig.IpMode == model.Static && r.appConfig.TapConfig.HwMac != 0 {
		if err := app.TunTapService.Start(); err != nil {
			app.Logger.Error("虚拟网卡初始化失败:", err.Error())
		} else {
			app.Logger.Info("虚拟网卡初始化成功,IP ", common.Uint32toIpV4(r.appConfig.TapConfig.IpAddr),
				" MAC ", common.Uint64ToMacStr(r.appConfig.TapConfig.HwMac))
		}
	}
	if err := r.Start(); err != nil {
		app.Logger.Error("运行时服务启动失败:", err.Error())
		return err
	}
	app.Logger.Info("运行时服务启动成功")

	if err := app.P2pService.Start(); err != nil {
		app.Logger.Error("P2P管理服务启动失败:", err.Error())
		return err
	}
	app.Logger.Info("P2P管理服务启动成功")
	return nil
}

// 状态定时检查
func (r *RuntimeService) stateCheck() {
	defer func() {
		if e := recover(); e != nil {
			app.Logger.Error("状态监测协程异常退出:", e)
		}
		r.running = false
	}()
	if !r.serverSock.IsConnected() {
		r.stopAllSockets()
		err := r.initServerSocket()
		if err != nil {
			app.Logger.Error("服务连接失败:", err)
		}
	}
	for r.running {
		select {
		case <-r.cancelContext.Done():
			r.stopAllSockets()
			return
		case <-time.After(3 * time.Second): //超时
			app.WailsApp.UpdateStats(r.stats)
			if !r.serverSock.IsConnected() {
				r.stopAllSockets()
				err := r.initServerSocket()
				if err != nil {
					app.Logger.Error("服务连接失败:", err)
				}
				continue
			} else if r.peerState != model.StateOk {
				_ = r.SendAuthRequest() // 重新注册
			}
		}
	}
	r.stopAllSockets()
}

func (r *RuntimeService) initServerSocket() error {
	if r.serverChannel != nil {
		_ = r.serverChannel.Close()
	}
	r.serverChannel = nil
	// 先用ping测试(对于关闭PING服务的服务器,请取消这段代码)
	if !common.Ping(r.appConfig.ServerIp, 1000) {
		r.SetPeerState(model.StateUnConn)
		return errors.New("无法PING通服务IP")
	}

	r.serverSock.Bootstrap = netty.NewBootstrap()
	// 子连接的流水线配置
	var initializer = func(channel netty.Channel) {
		channel.Pipeline().
			AddLast(netty.ReadIdleHandler(time.Duration(r.appConfig.Heartbeat) * time.Second)).
			AddLast(format.ProtobufCodec(1, uint32(r.appConfig.MaxPacketSize))).
			AddLast(NewServerHandler(r.serverSock))
	}
	r.serverSock.Bootstrap.ClientInitializer(initializer)
	url := fmt.Sprint("//", r.appConfig.ServerIp, ":", r.appConfig.ServerPort)
	chl, err := r.serverSock.Bootstrap.Transport(udp.New()).Connect(url, nil, transport.WithLocalAddr(&net.UDPAddr{IP: net.IPv4zero, Port: 0})) //
	if err != nil {
		r.SetPeerState(model.StateUnConn)
		r.serverChannel = nil
		return err
	} else {
		r.serverChannel = chl
		r.SetPeerState(model.StateUnAck)
		r.serverSock.Connected = true
	}
	ipAddr := strings.Split(chl.LocalAddr(), ":")
	r.updateLocalIp(ipAddr[0])
	r.updateLinkInfo(ipAddr[0])
	return nil
}

func (r *RuntimeService) updateLocalIp(ip string) {
	r.LocalIpStr = ip
	localIp := common.GetLocalIP(r.LocalIpStr) //
	if localIp != nil {
		if ip, e := common.IpV4toUint32(localIp.IP.To4().String()); e == nil {
			_, bits := localIp.Mask.Size()
			if bits == 32 {
				bits = 24
			}
			r.LocalIp = &protocol.IpNet{NetAddr: ip, NetBitLen: uint32(bits)}
		}
	}
}

func (r *RuntimeService) updateLinkInfo(ip string) {
	defer func() {
		recover()
	}()
	r.linkMode = model.LinkUnKnow
	var curFace net.Interface
	find := false
	iFaces, err := net.Interfaces()
	if err == nil {
		for _, iFace := range iFaces {
			addrs, err := iFace.Addrs()
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				if strings.Split(addr.String(), "/")[0] == ip {
					curFace = iFace
					if mode, err := common.GetIfType(iFace.Index); err == nil {
						r.linkMode = model.LinkMode(mode)
						find = true
						break
					}
				}
			}
			if find {
				break
			}
		}
	}
	switch r.linkMode {
	case model.LinkUnKnow:
		app.Logger.Debug(fmt.Sprintf("当前连接类型为:未知,信号值:未知"))
		r.linkQuality = 101
	case model.LinkETH:
		r.linkQuality = 101
		app.Logger.Debug(fmt.Sprintf("当前连接类型为:有线网,信号值:100"))
	case model.LinkWiFi:
		r.linkQuality, err = r.getWiFiQuality(curFace)
		app.Logger.Debug(fmt.Sprintf("当前连接类型为:WiFi,信号值:未知"))
	case model.LinkGprs:
		r.linkQuality = 101
		app.Logger.Debug(fmt.Sprintf("当前连接类型为:GPRS,信号值:未知"))
	}
}

func (r *RuntimeService) getWiFiQuality(face net.Interface) (uint32, error) {
	if w, err := wifi.New(); err == nil {
		iFaces, _ := w.Interfaces()
		for i := range iFaces {
			iFace := iFaces[i]
			if iFace.Name == face.Name {
				if s, err := w.StationInfo(iFace); err != nil {
					return 88, err
				} else {
					val := uint32((s[0].Signal + 120) / 90.0 * 100) // -30 ~ -120
					if val > 100 {
						val = 100
					} else if val < 0 {
						val = 0
					}
					return val, nil
				}
			}
		}
		return 101, nil
	} else {
		return 101, err
	}
}

func (r *RuntimeService) stopAllSockets() {
	if r.serverSock != nil {
		if r.serverSock.Bootstrap != nil {
			r.serverSock.Bootstrap.Stop()
		}
		r.serverSock.Connected = false
		r.SetPeerState(model.StateUnConn)
	}
	if r.serverChannel != nil {
		_ = r.serverChannel.Close()
		r.serverChannel = nil
	}
}
func (r *RuntimeService) initCrypt() error {
	if r.appConfig == nil || r.appConfig.PeerConfig == nil {
		return errors.New("加密器初始化失败:配置为空")
	}
	switch r.appConfig.PeerConfig.CryptType {
	case model.CryptNone:
		r.crypt = nil
		app.Logger.Info("正在使用无加密报文传输")
		break
	case model.CryptAES:
		r.crypt = common.NewAesCrypt(r.appConfig.PeerConfig.PeerPwd)
		if r.crypt == nil {
			return errors.New("AES加密器创建失败,将进行无加密传输")
		} else {
			app.Logger.Info("正在使用AES进行报文加解密")
		}
		break
	case model.CryptDES:
		r.crypt = common.NewDesCrypt(r.appConfig.PeerConfig.PeerPwd)
		if r.crypt == nil {
			return errors.New("DES加密器创建失败,将进行无加密传输")
		} else {
			app.Logger.Info("正在使用DES进行报文加解密")
		}
		break
	case model.CryptRSA:
		r.crypt = common.NewRsaCrypt()
		if r.crypt == nil {
			return errors.New("RSA加密器创建失败,将进行无加密传输")
		} else {
			app.Logger.Info("正在使用RSA进行报文加解密,请确认对端密钥文件一致")
		}
		break
	default:
		r.crypt = nil
		return errors.New("加密器创建失败,使用无加密报文传输")
	}
	return nil
}
func (r *RuntimeService) SetPeerState(state model.PeerState) {
	if r.peerState != state {
		r.peerState = state
		app.WailsApp.UpdateState(state)
	}
}

func (r *RuntimeService) PeerState() model.PeerState {
	return r.peerState
}

// 数据转发到服务器或目标端
func (r *RuntimeService) forwardMessage(msg *model.MessageOut) {
	if !r.running || msg == nil {
		return
	}
	if msg.Handler != nil { // 数据包
		msg.Handler.Write(msg.MsgContent)
	}
}

// 转发本地tun tap设备读取到的数据
func (r *RuntimeService) PostTunTapData(dstMac uint64, data []byte) error {
	if r.peerState != model.StateOk {
		return errors.New("服务未连接或未注册")
	}
	dataFrame := &protocol.MsgDataFrame{MsgType: protocol.MsgType_Msg_Packet, SrcMac: r.appConfig.TapConfig.HwMac, DstMac: dstMac, Data: data}
	msgOut := &model.MessageOut{MsgContent: dataFrame}

	if app.P2pService.TryForwardMessage(dstMac, dataFrame) { // P2P
		return nil
	} else if r.serverSock.Handler != nil { // 转发
		dataFrame.Token = r.serverSock.Token
		msgOut.Handler = r.serverSock.Handler
		r.forwardMessage(msgOut)
		return nil
	}
	return errors.New("没有找到有效的Sock")
}
func (r *RuntimeService) EncryptMsg(msg *protocol.MsgDataFrame, out []byte) (int, error) {
	if r.peerState < model.StateConnOk {
		return 0, errors.New("客户端连接未建立,不能传输信息")
	}
	if len(msg.Data) == 0 {
		return 0, errors.New("data is nil")
	}
	if r.crypt != nil {
		return r.crypt.Encode(msg.Data, out)
	}
	return 0, errors.New("crypt is nil")
}

func (r *RuntimeService) DecryptMsg(msg *protocol.MsgDataFrame, out []byte) (int, error) {
	if r.peerState < model.StateConnOk {
		return 0, errors.New("客户端连接未建立,不能传输信息")
	}
	if len(msg.Data) == 0 {
		return 0, errors.New("data is nil")
	}
	if r.crypt != nil {
		return r.crypt.Decode(msg.Data, out)
	}
	return 0, errors.New("crypt is nil")
}
func (r *RuntimeService) sendGratuitousArp() error {
	if r.peerState < model.StateConnOk {
		return errors.New("客户端未连接服务,不能发送信息")
	}
	if r.serverSock.Handler == nil {
		return errors.New("服务网络配置未初始化")
	}
	if r.appConfig.TapConfig.HwMac == 0 || r.appConfig.TapConfig.IpAddr == 0 {
		return errors.New("MAC地址或IP无效,不能发送ARP报文")
	}
	buffer := make([]byte, len(gratuitousArp))
	copy(buffer, gratuitousArp)
	mac := make([]byte, 8)
	binary.LittleEndian.PutUint64(mac, r.appConfig.TapConfig.HwMac)
	ip := make([]byte, 4)
	binary.BigEndian.PutUint32(ip, r.appConfig.TapConfig.IpAddr)
	copy(buffer[6:], mac[:6])
	copy(buffer[22:], mac[:6])
	copy(buffer[28:], ip)
	copy(buffer[38:], ip)
	return r.PostTunTapData(common.MacBroadcast, buffer)
}

func (r *RuntimeService) SendAuthRequest() error {
	if r.peerState < model.StateConnOk {
		return errors.New("客户端未连接服务,不能发送信息")
	}
	if r.serverSock.Handler == nil {
		return errors.New("服务网络配置未初始化")
	}
	groupPwd := ""
	if len(r.appConfig.PeerConfig.GroupPwd) > 0 {
		if pwd, e := common.AesEncrypt(r.appConfig.PeerConfig.GroupPwd, model.AESKey); e != nil {
			groupPwd = r.appConfig.PeerConfig.GroupPwd
		} else {
			groupPwd = base64.StdEncoding.EncodeToString([]byte(pwd))
		}
	}
	if r.LocalIp == nil {
		ipStr := strings.Split(r.serverSock.Handler.Channel().LocalAddr(), ":")
		r.updateLocalIp(ipStr[0])
		r.updateLinkInfo(ipStr[0])
	}
	authMsg := &protocol.MsgAuth{
		PeerName:    r.appConfig.PeerConfig.Name,
		Password:    groupPwd,
		PeerOs:      runtime.GOOS,
		LinkMode:    uint32(r.linkMode),
		LinkQuality: r.linkQuality,
		Group:       r.appConfig.PeerConfig.GroupName}
	if r.appConfig.TapConfig.HwMac == 0 {
		authMsg.AutoMac = true
	} else {
		authMsg.AutoMac = false
		authMsg.PeerMac = r.appConfig.TapConfig.HwMac
	}
	if r.appConfig.TapConfig.IpMode == model.AutoAssign {
		authMsg.AutoIp = true
	} else {
		authMsg.AutoIp = false
		authMsg.PeerAddr = &protocol.IpNet{NetAddr: r.appConfig.TapConfig.IpAddr, NetBitLen: r.appConfig.TapConfig.IpMask}
	}
	if r.LocalIp != nil {
		authMsg.InnerAddr = &protocol.IpNet{NetAddr: r.LocalIp.NetAddr, NetBitLen: r.LocalIp.NetBitLen}
	}
	packMsg := &protocol.MsgPeerFrame{PeerMac: r.appConfig.TapConfig.HwMac,
		MsgType: protocol.MsgType_Msg_Auth,
		Token:   0, MsgAuth: authMsg}
	msgOut := &model.MessageOut{Handler: r.serverSock.Handler, MsgContent: packMsg}
	r.forwardMessage(msgOut)
	r.SetPeerState(model.StateUnAck)
	return nil
}

func (r *RuntimeService) ProcessAuthResponse(ack *protocol.MsgAuthAck) error {
	if ack == nil {
		return errors.New("注册失败: 应答信息为空")
	}
	if ack.Sock != nil {
		r.externSock = ack.Sock
	}
	if ack.AuthRes >= 0 {
		r.serverSock.Token = ack.Token
		needSaveConfig := false
		if ack.AssignMac {
			if ack.PeerMac == 0 {
				r.SetPeerState(model.StateAuthFail)
				return errors.New("注册失败: 收到无效的MAC地址")
			} else {
				r.appConfig.TapConfig.HwMac = ack.PeerMac
				config.AppConfig.TapConfig.HwMac = ack.PeerMac
				needSaveConfig = true
			}
		}
		if ack.AssignIp {
			if ack.PeerAddr == nil || ack.PeerAddr.NetAddr == 0 || ack.PeerAddr.NetBitLen == 0 {
				r.SetPeerState(model.StateAuthFail)
				return errors.New("注册失败: 收到无效的IP地址")
			} else {
				r.appConfig.TapConfig.IpAddr = ack.PeerAddr.NetAddr
				r.appConfig.TapConfig.IpMask = ack.PeerAddr.NetBitLen
				config.AppConfig.TapConfig.IpAddr = ack.PeerAddr.NetAddr
				config.AppConfig.TapConfig.IpMask = ack.PeerAddr.NetBitLen
				needSaveConfig = true
			}
		}
		if needSaveConfig {
			_ = config.SaveAppConfig()
		}
		r.SetPeerState(model.StateOk)
		app.Logger.Info("服务注册成功")
		app.P2pService.ResetP2PSocks()
		if app.TunTapService.State() != model.TunTapRunning {
			if err := app.TunTapService.Start(); err != nil {
				r.SetPeerState(model.StateInitError)
				app.Logger.Error("虚拟网卡启动失败:", err)
			} else {
				app.Logger.Info("虚拟网卡初始化成功,IP ", common.Uint32toIpV4(r.appConfig.TapConfig.IpAddr),
					",MAC ", common.Uint64ToMacStr(r.appConfig.TapConfig.HwMac))
			}
			//_ = r.sendGratuitousArp()
		}
		_ = r.SendGroupPeersRequest(r.appConfig.PeerConfig.GroupName)
		return nil
	} else {
		r.SetPeerState(model.StateAuthFail)
		return errors.New(fmt.Sprintf("注册失败,返回错误码为%d", ack.AuthRes))
	}
}

func (r *RuntimeService) SendUnAuth() error {
	if r.peerState < model.StateConnOk {
		return errors.New("客户端未连接服务,不能发送信息")
	}
	if r.serverSock.Handler == nil {
		return errors.New("网络服务未完成初始化")
	}
	unAuthMsg := &protocol.MsgUnAuth{
		PeerMac: r.appConfig.TapConfig.HwMac,
		Group:   r.appConfig.PeerConfig.GroupName,
		Token:   r.serverSock.Token}
	packMsg := &protocol.MsgPeerFrame{PeerMac: r.appConfig.TapConfig.HwMac,
		MsgType: protocol.MsgType_Msg_UnAuth,
		Token:   r.serverSock.Token, MsgUnAuth: unAuthMsg}
	msgOut := &model.MessageOut{Handler: r.serverSock.Handler, MsgContent: packMsg}
	r.forwardMessage(msgOut)
	return nil
}

func (r *RuntimeService) ProcessPeerStateChanged(state *protocol.MsgState) error {
	if state == nil {
		return errors.New("状态变化消息为空")
	}
	// 上线
	if state.Online {
		if state.PeerInfo != nil {
			r.groupPeers.Store(state.PeerMac, state.PeerInfo)
		}
	} else {
		if v, ok := r.groupPeers.Load(state.PeerMac); ok {
			p := v.(*protocol.PeerInfo)
			p.Online = false
		}
	}
	app.WailsApp.UpdatePeers()
	_ = app.P2pService.PeerStateChanged(state.PeerMac, state.Online)
	_ = app.SerialService.PeerStateChanged(state.PeerMac, state.Online)
	return nil
}

func (r *RuntimeService) sendConfig(conf *protocol.MsgConfig, result chan error) error {
	if r.peerState < model.StateConnOk {
		err := errors.New("客户端未连接服务,不能发送信息")
		return err
	}
	if r.serverSock.Handler == nil {
		err := errors.New("网络服务未完成初始化")
		return err
	}
	if conf == nil {
		err := errors.New("网络服务未完成初始化")
		return err
	}
	packMsg := &protocol.MsgPeerFrame{PeerMac: r.appConfig.TapConfig.HwMac,
		MsgType: protocol.MsgType_Msg_Config,
		Token:   r.serverSock.Token, MsgConfig: conf}
	msgOut := &model.MessageOut{Handler: r.serverSock.Handler, MsgContent: packMsg}
	r.forwardMessage(msgOut)
	r.configSend[conf.DstMac] = result
	return nil
}

func (r *RuntimeService) UpdateConfig(peer *model.PeerInfo) map[string]interface{} {
	if peer == nil {
		return map[string]interface{}{"result": false, "tip": "参数为空"}
	}

	mac, e := strconv.ParseUint(peer.PeerMac, 10, 64)
	if e != nil {
		return map[string]interface{}{"result": false, "tip": "无效的终端MAC地址:" + peer.PeerMac}
	}
	ip, ipNet, err := net.ParseCIDR(peer.InterAddr)
	if err != nil {
		return map[string]interface{}{"result": false, "tip": "无效的终端内网地址"}
	}
	netAddr, e := common.IpV4toUint32(ip.String())
	if e != nil {
		return map[string]interface{}{"result": false, "tip": "无效的终端内网地址"}
	}
	bitLen, _ := ipNet.Mask.Size()
	res := make(chan error)
	if err := r.sendConfig(&protocol.MsgConfig{SrcMac: config.AppConfig.TapConfig.HwMac, DstMac: mac, NewName: peer.PeerName,
		InnerAddr: &protocol.IpNet{NetAddr: netAddr, NetBitLen: uint32(bitLen)}}, res); err != nil {
		return map[string]interface{}{"result": false, "tip": err.Error()}
	}
	select {
	case result := <-res:
		delete(r.configSend, mac)
		if result == nil {
			return map[string]interface{}{"result": true, "tip": "参数设置成功"}
		} else {
			return map[string]interface{}{"result": false, "tip": result.Error()}
		}
	case <-time.After(2 * time.Second):
		delete(r.configSend, mac)
		return map[string]interface{}{"result": false, "tip": "设置超时,没有收到应答"}
	}
}

func (r *RuntimeService) ProcessConfig(conf *protocol.MsgConfig) error {
	if conf == nil {
		return errors.New("配置信息为空")
	}
	msgAck := &protocol.MsgConfigAck{SrcMac: r.appConfig.TapConfig.HwMac, DstMac: conf.SrcMac}
	packMsg := &protocol.MsgPeerFrame{PeerMac: r.appConfig.TapConfig.HwMac,
		MsgType: protocol.MsgType_Msg_ConfigAck, MsgConfigAck: msgAck,
		Token: r.serverSock.Token}
	msgOut := &model.MessageOut{Handler: r.serverSock.Handler, MsgContent: packMsg}
	msgAck.Tip = "该终端参数不允许远程修改"
	msgAck.IsOk = false
	r.forwardMessage(msgOut)
	return errors.New("不允许远程设置参数")
}

func (r *RuntimeService) ProcessConfigAck(msgAck *protocol.MsgConfigAck) error {
	if msgAck == nil {
		return errors.New("配置应答消息为空")
	}
	if res, ok := r.configSend[msgAck.SrcMac]; ok {
		if msgAck.IsOk {
			res <- nil
		} else {
			res <- errors.New(msgAck.Tip)
		}
	}
	// 通知设置结果
	return nil
}

func (r *RuntimeService) SendPing() error {
	if r.peerState < model.StateConnOk {
		return errors.New("客户端未连接服务,不能发送信息")
	}
	if r.serverSock.Handler == nil {
		return errors.New("网络服务未完成初始化")
	}
	r.updateLinkInfo(r.LocalIpStr)
	pingMsg := &protocol.MsgPing{
		PeerName:    r.appConfig.PeerConfig.Name,
		PeerAddr:    &protocol.IpNet{NetAddr: r.appConfig.TapConfig.IpAddr, NetBitLen: r.appConfig.TapConfig.IpMask},
		LinkMode:    uint32(r.linkMode),
		LinkQuality: r.linkQuality}
	if r.LocalIp != nil {
		pingMsg.InnerAddr = &protocol.IpNet{NetAddr: r.LocalIp.NetAddr, NetBitLen: r.LocalIp.NetBitLen}
	}
	pingMsg.Stats = r.stats
	packMsg := &protocol.MsgPeerFrame{PeerMac: r.appConfig.TapConfig.HwMac,
		MsgType: protocol.MsgType_Msg_Ping,
		Token:   r.serverSock.Token, MsgPing: pingMsg}
	msgOut := &model.MessageOut{Handler: r.serverSock.Handler, MsgContent: packMsg}
	r.forwardMessage(msgOut)
	return nil
}

func (r *RuntimeService) ProcessPong(pong *protocol.MsgPong) error {
	if pong == nil {
		return errors.New("心跳消息为空")
	}
	if pong.Sock != nil {
		r.externSock = pong.Sock
	}
	return nil
}

func (r *RuntimeService) SendGroupPeersRequest(groupName string) error {
	if r.peerState < model.StateConnOk {
		return errors.New("客户端未连接服务,不能发送信息")
	}
	if r.serverSock.Handler == nil {
		return errors.New("网络服务未完成初始化")
	}

	common.ClearMap(r.groupPeers)

	msg := &protocol.MsgGroupPeersRequest{SrcMac: r.appConfig.TapConfig.HwMac, GroupName: groupName}
	packMsg := &protocol.MsgPeerFrame{PeerMac: r.appConfig.TapConfig.HwMac,
		MsgType: protocol.MsgType_Msg_GroupPeersRequest,
		Token:   r.serverSock.Token, MsgGroupPeer: msg}
	msgOut := &model.MessageOut{Handler: r.serverSock.Handler, MsgContent: packMsg}
	r.forwardMessage(msgOut)
	return nil
}

func (r *RuntimeService) ProcessGroupPeersResponse(response *protocol.MsgGroupPeersResponse) error {
	if response == nil {
		return errors.New("group peers request failed:response msg is nil")
	}
	if r.groupPeerCookie != response.Cookie || r.groupPeers == nil {
		common.ClearMap(r.groupPeers)
		r.groupPeerCookie = response.Cookie
	}
	if response.PeerInfo != nil {
		for _, p := range response.PeerInfo {
			r.groupPeers.Store(p.PeerMac, p)
		}
	}
	app.WailsApp.UpdatePeers()
	return nil
}

func (r *RuntimeService) SendPeerLinksRequest(dstMac uint64) error {
	if r.peerState < model.StateConnOk {
		return errors.New("客户端未连接服务,不能发送信息")
	}
	if r.serverSock.Handler == nil {
		return errors.New("网络服务未完成初始化")
	}
	msg := &protocol.MsgPeerLinksRequest{SrcMac: r.appConfig.TapConfig.HwMac, DstMac: dstMac}

	packMsg := &protocol.MsgPeerFrame{PeerMac: r.appConfig.TapConfig.HwMac,
		MsgType: protocol.MsgType_Msg_PeerLinksRequest,
		Token:   r.serverSock.Token, MsgLinks: msg}
	msgOut := &model.MessageOut{Handler: r.serverSock.Handler, MsgContent: packMsg}
	r.forwardMessage(msgOut)
	return nil
}

func (r *RuntimeService) ProcessPeerLinksRequest(request *protocol.MsgPeerLinksRequest) error {
	if r.serverSock.Handler == nil {
		return errors.New("网络服务未完成初始化")
	}
	if r.LocalIp == nil {
		return errors.New("本地IP获取失败")
	}
	links := make([]*protocol.LinkInfo, 0)

	subMask := uint32(0xFFFFFFFF)
	segIp := r.LocalIp.NetAddr & (subMask << (32 - r.LocalIp.NetBitLen))
	rand.Seed(time.Now().Unix())
	cookie := rand.Uint32() + 1
	// arp.Search(ip) 获取mac
	for ipStr := range arp.Table() {
		if ip, err := common.IpV4toUint32(ipStr); err == nil {
			if segIp == (ip & (subMask << (32 - r.LocalIp.NetBitLen))) {
				links = append(links, &protocol.LinkInfo{Addr: ip})
			}
		}
	}
	// 每包50
	pkg := len(links) / 50
	if len(links)%50 > 0 {
		pkg += 1
	}
	for i := 0; i < pkg; i++ {
		msg := &protocol.MsgPeerLinksResponse{SrcMac: r.appConfig.TapConfig.HwMac, DstMac: request.SrcMac, Cookie: cookie}
		if i == pkg-1 {
			msg.LinkInfo = links[i*50:]
		} else {
			msg.LinkInfo = links[i*50 : (i+1)*50]
		}
		packMsg := &protocol.MsgPeerFrame{PeerMac: r.appConfig.TapConfig.HwMac,
			MsgType: protocol.MsgType_Msg_PeerLinksResponse,
			Token:   r.serverSock.Token, MsgLinksAck: msg}
		msgOut := &model.MessageOut{Handler: r.serverSock.Handler, MsgContent: packMsg}
		r.forwardMessage(msgOut)
	}
	return nil
}

func (r *RuntimeService) ProcessPeerLinksResponse(response *protocol.MsgPeerLinksResponse) error {
	if response == nil {
		return errors.New("接入设备请求应答消息为空")
	}
	if response.LinkInfo == nil {
		return errors.New("接入设备请求应答结果为空")
	}

	r.linkMutex.Lock()
	if c, ok := r.linkInfoReqCookie[response.SrcMac]; !ok || c != response.Cookie {
		r.linkInfoReqCookie[response.SrcMac] = response.Cookie
		r.linkInfos[response.SrcMac] = response.LinkInfo
	} else {
		if links, exist := r.linkInfos[response.SrcMac]; exist {
			r.linkInfos[response.SrcMac] = append(links, response.LinkInfo...)
		} else {
			r.linkInfos[response.SrcMac] = response.LinkInfo
		}
	}
	r.linkMutex.Unlock()
	return nil
}

// 不做等待 直接返回 (收到应答再推送)
func (r *RuntimeService) GetGroupPeers(group string, request bool) []*protocol.PeerInfo {
	if len(group) == 0 {
		return nil
	}
	peers := make([]*protocol.PeerInfo, 0)
	if !request {
		r.groupPeers.Range(func(key, value interface{}) bool {
			v := value.(*protocol.PeerInfo)
			peers = append(peers, v)
			return true
		})
		return peers
	}
	_ = r.SendGroupPeersRequest(group)
	return peers
}

func (r *RuntimeService) FindPeer(mac uint64) *protocol.PeerInfo {
	if p, ok := r.groupPeers.Load(mac); ok {
		return p.(*protocol.PeerInfo)
	} else {
		return nil
	}
}

func (r *RuntimeService) GetLinkInfos(mac uint64) []*protocol.LinkInfo {
	r.linkMutex.Lock()
	delete(r.linkInfos, mac)
	r.linkMutex.Unlock()
	err := r.SendPeerLinksRequest(mac)
	if err != nil {
		return nil
	}
	count := 0
	lastLen := 0
	for count < 15 {
		time.Sleep(100 * time.Millisecond)
		count++
		curLen := len(r.linkInfos[mac])
		if curLen > 0 {
			if lastLen == curLen {
				return r.linkInfos[mac]
			} else {
				lastLen = curLen
			}
		}
	}
	return nil
}

func (r *RuntimeService) SetStats(size uint64, rx bool, p2p bool) {
	if r.stats == nil || r.peerState < model.StateConnOk {
		return
	}
	if rx {
		if p2p {
			r.stats.P2PReceive += size
		} else {
			r.stats.TransReceive += size
		}
	} else {
		if p2p {
			r.stats.P2PSend += size
		} else {
			r.stats.TransSend += size
		}
	}
}

func (r *RuntimeService) GetStats() *protocol.Statistics {
	return r.stats
}

func (r *RuntimeService) AddRoute(mac uint64) bool {
	p := r.FindPeer(mac)
	if p == nil {
		app.Logger.Error("路由添加失败:没有找到相应客户端")
		return false
	}
	r.CleanRoutes()
	route := &model.RouteInfo{}
	subMask := uint32(0xFFFFFFFF)
	segIp := p.InterAddr & (subMask << (32 - p.InterNetBitLen))
	route.Destination = segIp
	route.Mask = subMask << (32 - p.InterNetBitLen)
	route.Gateway = p.NetAddr

	err := app.TunTapService.AddRoute(route.GetDst(), route.GetMask(), route.GetGw())
	if err != nil {
		app.Logger.Error("路由添加失败:", err)
		return false
	}
	r.lastRoute = route
	return true
}

func (r *RuntimeService) CleanRoutes() {
	if r.lastRoute != nil {
		_ = app.TunTapService.DelRoute(r.lastRoute.GetDst(), r.lastRoute.GetMask(), r.lastRoute.GetGw())
	}
	r.lastRoute = nil
	return
}
