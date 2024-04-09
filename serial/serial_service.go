package serial

import (
	"errors"
	"fmt"
	"sync"
	"time"
	"vilan/app"
	"vilan/common"
	"vilan/config"
	"vilan/netty"
	"vilan/netty/transport/tcp"
	"vilan/serial/protocol"
	serial2 "vilan/sys/serial"
)

type PortService struct {
	isRunning bool
	bootstrap netty.Bootstrap
	serverCtx *sync.Map //map[uint64]*ServerContext // 远端的接入的连接

	clientCtx *sync.Map //map[uint64]*ClientContext // 本地向远端的连接

	localId uint64 // 唯一ID 本地mac
}

func NewPortService() (n *PortService) {
	return &PortService{isRunning: false, localId: config.AppConfig.TapConfig.HwMac}
}

func (s *PortService) Start() error {
	if s.bootstrap != nil {
		s.Stop()
	}
	defer func() {
		if err := recover(); err != nil {
			return
		}
	}()
	s.bootstrap = netty.NewBootstrap()
	// 子连接的流水线配置
	var childInitializer = func(channel netty.Channel) {
		channel.Pipeline().
			AddLast(message.ProtobufCodec(0, 2048)).
			AddLast(netty.ReadIdleHandler(30 * time.Second)).
			AddLast(NewServerHandler(s))
	}
	var option = &tcp.Options{
		Timeout:         time.Second * 5,
		KeepAlive:       true,
		KeepAlivePeriod: time.Minute, // tcp ping 等待间隔
		Linger:          0,
		NoDelay:         true,
	}
	// 创建Bootstrap & 监听端口 & 接受连接
	//n.bootstrap.Channel(netty.NewBufferedChannel(config.AppConfig.PacketNum, int(config.AppConfig.MaxPacketSize)))
	s.bootstrap.ChildInitializer(childInitializer).ClientInitializer(childInitializer)
	s.bootstrap.ChannelExecutor(netty.NewFlexibleChannelExecutor(2, 1, 5))
	s.bootstrap.Transport(tcp.New()).Listen(fmt.Sprintf(":%d", 48532), tcp.WithOptions(option))
	s.serverCtx = &sync.Map{}
	s.clientCtx = &sync.Map{}
	s.isRunning = true
	go s.stateCheck()
	return nil
}

func (s *PortService) Stop() {
	s.serverCtx.Range(func(key, value interface{}) bool {
		v := value.(*ServerContext)
		_ = v.Stop()
		return true
	})
	common.ClearMap(s.serverCtx)
	s.clientCtx.Range(func(key, value interface{}) bool {
		v := value.(*ClientContext)
		_ = v.Stop()
		return true
	})
	common.ClearMap(s.clientCtx)

	time.Sleep(1 * time.Second)
	s.bootstrap.Stop()
	s.bootstrap = nil
}

func (s *PortService) stateCheck() {
	for s.isRunning {
		toRemove := make([]uint64, 0)
		now := time.Now().Unix()
		s.serverCtx.Range(func(key, value interface{}) bool {
			v := value.(*ServerContext)
			if now-v.LastReceive > 65 {
				_ = v.Stop()
				toRemove = append(toRemove, key.(uint64))
			}
			return true
		})
		for _, key := range toRemove {
			s.serverCtx.Delete(key)
		}
		toRemove = make([]uint64, 0)
		s.clientCtx.Range(func(key, value interface{}) bool {
			v := value.(*ClientContext)
			if now-v.LastReceive > 65 {
				_ = v.Stop()
				toRemove = append(toRemove, key.(uint64))
			}
			return true
		})
		for _, key := range toRemove {
			s.clientCtx.Delete(key)
		}
		<-time.After(3 * time.Second)
	}
}

func (s *PortService) SetServerHandle(id uint64, ctx netty.HandlerContext) {
	serCtx := NewServerContext(s, ctx)
	if serCtx.Start() == nil {
		s.serverCtx.Store(id, serCtx)
	}
}
func (s *PortService) RemoveServerHandler(id uint64) {
	if v, ok := s.serverCtx.Load(id); ok {
		ctx := v.(*ServerContext)
		_ = ctx.Stop()
		s.serverCtx.Delete(id)
	}
}

func (s *PortService) ProcessClientMessage(dstId uint64, msg *message.MsgClientFrame, ctx netty.HandlerContext) {
	if msg == nil {
		return
	}
	switch msg.MsgType {
	case message.MsgType_Msg_Auth:
		if msg.MsgAuth == nil {
			return
		}
		if app.RuntimeService.FindPeer(dstId) == nil {
			return
		}
		msgAuthAck := &message.MsgAuthAck{Tip: "注册成功"}
		serMsg := &message.MsgServerFrame{MsgType: message.MsgType_Msg_AuthAck, SrcId: s.localId, MsgAuthAck: msgAuthAck}
		ctx.Write(serMsg)
		s.SetServerHandle(dstId, ctx)
		break
	case message.MsgType_Msg_UnAuth:
		s.RemoveServerHandler(dstId)
		break
	case message.MsgType_Msg_Ping:
		if s, ok := s.serverCtx.Load(dstId); ok {
			serCtx := s.(*ServerContext)
			serCtx.LastReceive = time.Now().Unix()
		}
		sMsg := &message.MsgServerFrame{MsgType: message.MsgType_Msg_Pong, SrcId: s.localId}
		ctx.Write(sMsg)
		break
	default:
		if s, ok := s.serverCtx.Load(dstId); ok {
			serCtx := s.(*ServerContext)
			serCtx.LastReceive = time.Now().Unix()
			_ = serCtx.processMsg(msg)
		}
	}
}

func (s *PortService) ConnectRemote(dstId uint64) map[string]interface{} {
	peer := app.RuntimeService.FindPeer(dstId) // 此处使用内部，独立使用时  改为外部链接地址
	if peer == nil {
		return map[string]interface{}{"result": false, "tip": "没有找到指定客户端"}
	}
	ip := fmt.Sprintf("%d.%d.%d.%d", peer.NetAddr>>24, (peer.NetAddr>>16)&0xFF, (peer.NetAddr>>8)&0xFF, peer.NetAddr&0xFF)
	url := fmt.Sprint("//", ip, ":48532")
	clientCtx := NewClientContext(s, url)
	err := clientCtx.Start()
	if err != nil {
		return map[string]interface{}{"result": false, "tip": "远端网络连接失败"}
	} else {
		if c, ok := s.clientCtx.Load(dstId); ok {
			client := c.(*ClientContext)
			_ = client.Stop()
			s.clientCtx.Delete(dstId)
		}
		s.clientCtx.Store(dstId, clientCtx)
	}
	result := clientCtx.WaitAuth()
	if result["result"] != true {
		s.clientCtx.Delete(dstId)
	}
	return result
}

// 重新连接
func (s *PortService) ReConnectRemote(dstId uint64) map[string]interface{} {
	peer := app.RuntimeService.FindPeer(dstId) // 此处使用内部，独立使用时  改为外部链接地址
	if peer == nil {
		return map[string]interface{}{"result": false, "tip": "没有找到指定客户端"}
	}
	ip := fmt.Sprintf("%d.%d.%d.%d", peer.NetAddr>>24, (peer.NetAddr>>16)&0xFF, (peer.NetAddr>>8)&0xFF, peer.NetAddr&0xFF)
	url := fmt.Sprint("//", ip, ":48532")
	clientContext := NewClientContext(s, url)
	err := clientContext.Start()
	portContexts := make(map[string]*PortContext, 0)
	// todo 记录失败次数
	if err != nil {
		return map[string]interface{}{"result": false, "tip": "远端网络连接失败"}
	} else {
		if c, ok := s.clientCtx.Load(dstId); ok {
			client := c.(*ClientContext)
			_ = client.Stop()
			s.clientCtx.Delete(dstId)
			client.portCtx.Range(func(key, value any) bool {
				portCtx := value.(*PortContext)
				portContexts[key.(string)] = portCtx
				return true
			})
		}
		s.clientCtx.Store(dstId, clientContext)
	}
	result := clientContext.WaitAuth()
	if result["result"] != true {
		s.clientCtx.Delete(dstId)
	} else {
		if len(portContexts) > 0 {
			for i := range portContexts {
				conf := portContexts[i].Config
				if conf == nil {
					continue
				}
				s.OpenPort(dstId, conf.RemoteName, conf.Name, conf.UserPortName, conf)
			}
		}
	}
	return result
}

func (s *PortService) DisConnectRemote(dstId uint64) map[string]interface{} {
	if c, ok := s.clientCtx.Load(dstId); ok {
		ctx := c.(*ClientContext)
		_ = ctx.Stop()
		s.clientCtx.Delete(dstId)
	}
	return map[string]interface{}{"result": true, "tip": "连接关闭成功"}
}

// 打开远程串口(同时打开本地串口)
func (s *PortService) OpenPort(dstId uint64, remote string, local string, user string, config *serial2.Config) map[string]interface{} {
	if c, ok := s.clientCtx.Load(dstId); ok {
		ctx := c.(*ClientContext)
		if config != nil {
			config.ReadTimeout = 0 // 阻塞
		}
		res := ctx.OpenPort(remote, local, user, config)
		if res["timeout"] == nil {
			return res
		}
		s.deleteContext(dstId)
	}
	result := s.ConnectRemote(dstId)
	if result["result"] != true {
		return map[string]interface{}{"result": false, "tip": "远端网络连接失败"}
	} else {
		if c0, ok := s.clientCtx.Load(dstId); ok {
			ctx := c0.(*ClientContext)
			if config != nil {
				config.ReadTimeout = 0 // 阻塞
			}
			return ctx.OpenPort(remote, local, user, config)
		} else {
			return map[string]interface{}{"result": false, "tip": "远端网络连接失败"}
		}
	}
}

// 关闭远程串口(同时关闭本地串口)
func (s *PortService) ClosePort(dstId uint64, name string) map[string]interface{} {
	if c, ok := s.clientCtx.Load(dstId); ok {
		ctx := c.(*ClientContext)
		result := ctx.ClosePort(name)
		// 已没有打开的串口
		if common.MapLength(ctx.portCtx) <= 0 {
			s.DisConnectRemote(dstId)
		}
		return result
	} else {
		return map[string]interface{}{"result": false, "tip": "远端未建立网络连接"}
	}
}

// 获取远程串口配置
func (s *PortService) GetPortConfig(dstId uint64, name string) map[string]interface{} {
	if c, ok := s.clientCtx.Load(dstId); ok {
		ctx := c.(*ClientContext)
		res := ctx.GetPortConfig(name)
		if res["timeout"] == nil {
			return res
		}
		s.deleteContext(dstId)
	}
	result := s.ConnectRemote(dstId)
	if result["result"] != true {
		return map[string]interface{}{"result": false, "tip": "远端网络连接失败"}
	} else {
		if c0, ok := s.clientCtx.Load(dstId); ok {
			ctx := c0.(*ClientContext)
			return ctx.GetPortConfig(name)
		} else {
			return map[string]interface{}{"result": false, "tip": "远端网络连接失败"}
		}
	}
}
func (s *PortService) SetPortConfig(dstId uint64, remote string, config *serial2.Config) map[string]interface{} {
	if c, ok := s.clientCtx.Load(dstId); ok {
		ctx := c.(*ClientContext)
		res := ctx.SetPortConfig(remote, config)
		if res["timeout"] == nil {
			return res
		}
		s.deleteContext(dstId)
	}
	result := s.ConnectRemote(dstId)
	if result["result"] != true {
		return map[string]interface{}{"result": false, "tip": "远端网络连接失败"}
	} else {
		if c0, ok := s.clientCtx.Load(dstId); ok {
			ctx := c0.(*ClientContext)
			return ctx.SetPortConfig(remote, config)
		} else {
			return map[string]interface{}{"result": false, "tip": "远端网络连接失败"}
		}
	}
}

func (s *PortService) GetConnectedPorts(dstId uint64) map[string]interface{} {
	if c, ok := s.clientCtx.Load(dstId); ok {
		ctx := c.(*ClientContext)
		res := ctx.GetConnectedPorts()
		if res["timeout"] == nil {
			return res
		}
		s.deleteContext(dstId)
	}
	result := s.ConnectRemote(dstId)
	if result["result"] != true {
		return map[string]interface{}{"result": false, "tip": "远端网络连接失败"}
	} else {
		if c0, ok := s.clientCtx.Load(dstId); ok {
			ctx := c0.(*ClientContext)
			return ctx.GetConnectedPorts()
		} else {
			return map[string]interface{}{"result": false, "tip": "远端网络连接失败"}
		}
	}
}

// GetRemotePortList 获取远程串口列表
func (s *PortService) GetRemotePortList(dstId uint64) map[string]interface{} {
	if c, ok := s.clientCtx.Load(dstId); ok {
		ctx := c.(*ClientContext)
		res := ctx.GetPortList()
		if res["timeout"] == nil {
			return res
		}
		s.deleteContext(dstId)
	}
	result := s.ConnectRemote(dstId)
	if result["result"] != true {
		return map[string]interface{}{"result": false, "tip": "远端网络连接失败"}
	} else {
		if c0, ok := s.clientCtx.Load(dstId); ok {
			ctx := c0.(*ClientContext)
			return ctx.GetPortList()
		} else {
			return map[string]interface{}{"result": false, "tip": "远端网络连接失败"}
		}
	}
}

func (s *PortService) GetLocalPortList() map[string]interface{} {
	list, err := serial2.GetPortsList()
	if err != nil {
		return map[string]interface{}{"result": false, "tip": "串口列表获取失败"}
	} else {
		return map[string]interface{}{"result": true, "tip": "串口列表获取成功", "list": list}
	}
}

func (s *PortService) PeerStateChanged(mac uint64, _ bool) error {
	s.deleteContext(mac)
	return nil
}

func (s *PortService) deleteContext(mac uint64) {
	if c, ok := s.clientCtx.Load(mac); ok {
		ctx := c.(*ClientContext)
		if ctx.handler != nil {
			ctx.handler.Close(nil)
		}
		_ = ctx.Stop()
		s.clientCtx.Delete(mac)
	}
	if c, ok := s.serverCtx.Load(mac); ok {
		ctx := c.(*ServerContext)
		if ctx.handler != nil {
			ctx.handler.Close(nil)
		}
		_ = ctx.Stop()
		s.serverCtx.Delete(mac)
	}
}

func (s *PortService) ProcessServerMessage(dstId uint64, msg *message.MsgServerFrame, ctx netty.HandlerContext) {
	if msg == nil {
		return
	}
	switch msg.MsgType {
	case message.MsgType_Msg_AuthAck:
		if c, ok := s.clientCtx.Load(dstId); ok {
			client := c.(*ClientContext)
			client.LastReceive = time.Now().Unix()
			client.SetHandler(ctx)
			_ = client.processMsg(msg)
		}
		break
	case message.MsgType_Msg_Ping:
		if c, ok := s.clientCtx.Load(dstId); ok {
			client := c.(*ClientContext)
			client.LastReceive = time.Now().Unix()
		}
		cMsg := &message.MsgClientFrame{MsgType: message.MsgType_Msg_Pong, SrcId: s.localId}
		ctx.Write(cMsg)
		break
	default:
		if c, ok := s.clientCtx.Load(dstId); ok {
			client := c.(*ClientContext)
			client.LastReceive = time.Now().Unix()
			_ = client.processMsg(msg)
		}
	}
}

func (s *PortService) GetLocalPorts() ([]string, error) {
	return serial2.GetPortsList()
}

func (s *PortService) GetLocalPortConfig(name string) (*serial2.Config, error) {
	if configs, err := serial2.LoadConfig(); err != nil {
		return nil, err
	} else {
		if configs == nil {
			return nil, errors.New("serial config not found")
		}
		return configs[name], nil
	}
}
