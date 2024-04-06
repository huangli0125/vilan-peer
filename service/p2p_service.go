package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"syscall"
	"time"
	"vilan/app"
	"vilan/common"
	"vilan/config"
	"vilan/model"
	"vilan/netty"
	"vilan/netty/codec/format"
	"vilan/netty/transport"
	"vilan/netty/transport/udp"
	"vilan/protocol"
)

// p2p对接流程为：
// 收到DataPack报文，判断是否需要P2P（根据NAT、是否已建立P2P、是否在尝试P2P）
// 如果需要则绑定本地任意端口连接服务器端口
// 发送触发对端P2P启动报文
// 对端收到触发报文,也上报自己的触发报文
// 服务器收到两边的公网地址后，分别发送对端的地址
// 两边收到对端地址，开始尝试P2P

type P2pService struct {
	running       bool
	cancelContext context.Context
	cancelFunc    context.CancelFunc

	p2pFailedTime  int64
	punchSocks     *sync.Map //map[uint64]*model.PunchSockContext // 启动后 p2pFailedTime S 没有移除就是已经失败，mac移入失败名
	p2pTrySocks    *sync.Map //map[uint64]*model.PeerSockContext  // 正在尝试建立的p2p
	p2pSocks       *sync.Map //map[uint64]*model.PeerSockContext // 已建立的p2p 连接
	hisP2PFailInfo *sync.Map //map[uint64]*model.PunchFailInfo // 尝试失败的p2p
}

func NewP2pService() *P2pService {
	r := &P2pService{running: false}
	return r
}
func (r *P2pService) Start() (err error) {
	if r.running {
		return
	}
	r.running = true
	r.cancelContext, r.cancelFunc = context.WithCancel(context.Background()) // 全局取消
	r.punchSocks = &sync.Map{}
	r.p2pTrySocks = &sync.Map{}
	r.p2pSocks = &sync.Map{}
	r.hisP2PFailInfo = &sync.Map{}
	r.p2pFailedTime = int64(config.AppConfig.Heartbeat)
	go r.stateCheck()
	return err
}

func (r *P2pService) Stop() {
	r.running = false
	r.cancelFunc() // 取消
}

// 状态定时检查
func (r *P2pService) stateCheck() {
	defer func() {
		if e := recover(); e != nil {
			app.Logger.Error("状态监测协程异常退出:", e)
		}
		r.running = false
	}()

	for r.running {
		select {
		case <-r.cancelContext.Done(): //拿到锁
			r.stopP2pSockets()
			return
		case <-time.After(3 * time.Second): //超时
			now := time.Now().Unix()
			toRemoveP2P := make([]*model.PeerSockContext, 0)
			r.p2pSocks.Range(func(key, value interface{}) bool {
				v := value.(*model.PeerSockContext)
				if uint32(now-v.LastReceive) > v.Heartbeat+2 {
					if v.PingTryCount >= 3 || v.Handler == nil {
						v.Bootstrap.Stop()
						v.Connected = false
						toRemoveP2P = append(toRemoveP2P, v)
					} else {
						msgFrame := &protocol.MsgDataFrame{SrcMac: config.AppConfig.TapConfig.HwMac, DstMac: v.PeerMac, MsgType: protocol.MsgType_Msg_Ping}
						v.PingTryCount++
						v.Handler.Write(msgFrame)
					}
				} else if uint32(now-v.LastReceive) > config.AppConfig.Offline {
					v.Bootstrap.Stop()
					v.Connected = false
					toRemoveP2P = append(toRemoveP2P, v)
				}
				return true
			})
			if len(toRemoveP2P) > 0 {
				for _, u := range toRemoveP2P {
					r.p2pSocks.Delete(u.PeerMac)
				}
				app.WailsApp.UpdatePeers()
			}

			toRemoveP2PTry := make([]*model.PeerSockContext, 0)
			r.p2pTrySocks.Range(func(key, value interface{}) bool {
				v := value.(*model.PeerSockContext)
				if v.TryConnCount >= config.AppConfig.P2pTryCount {
					v.Bootstrap.Stop()
					v.Connected = false
					toRemoveP2PTry = append(toRemoveP2PTry, v)
				} else {
					err := r.SendP2PTry(v.PeerMac)
					if v.Handler != nil {
						app.Logger.Debug("尝试P2P发数据:", v.Handler.Channel().RemoteAddr(), err)
					} else {
						app.Logger.Debug("尝试P2P发数据:", err)
					}
				}
				return true
			})
			if len(toRemoveP2PTry) > 0 {
				for _, u := range toRemoveP2PTry {
					r.saveFailPunchInfo(u.PeerMac)
					r.p2pTrySocks.Delete(u.PeerMac)
				}
			}
		}
	}
	r.stopP2pSockets()
}

func (r *P2pService) stopP2pSockets() {
	keyToDel := make([]interface{}, 0)
	if r.punchSocks != nil {
		r.punchSocks.Range(func(key, value interface{}) bool {
			keyToDel = append(keyToDel, key)
			switch v := value.(type) {
			case model.PunchSockContext:
				if v.Bootstrap != nil {
					v.Bootstrap.Stop()
					v.Connected = false
				}
			default:
			}
			return true
		})
	}
	r.p2pSocks.Range(func(key, value interface{}) bool {
		v := value.(*model.PeerSockContext)
		if v.Bootstrap != nil {
			v.Bootstrap.Stop()
			v.Connected = false
		}
		return true
	})
	common.ClearMap(r.p2pSocks)

	r.p2pTrySocks.Range(func(key, value interface{}) bool {
		v := value.(*model.PeerSockContext)
		if v.Bootstrap != nil {
			v.Bootstrap.Stop()
			v.Connected = false
		}
		return true
	})
	common.ClearMap(r.p2pTrySocks)
}

func (r *P2pService) TryForwardMessage(mac uint64, msg *protocol.MsgDataFrame) bool {
	if !r.running || msg == nil {
		return false
	}
	// 组播广播包通过公网服务转发
	if !common.IsUniCast(mac) {
		return false
	}
	if v, ok := r.p2pSocks.Load(mac); ok { // P2P
		sock := v.(*model.PeerSockContext)
		if sock.Handler != nil {
			sock.Handler.Write(msg)
			return true
		}
		return false
	}
	// 判断是否需要启动 p2p
	if r.isNeedP2P(mac) {
		if err := r.StartPunch(mac, false); err != nil {
			app.Logger.Debug("P2P服务启动失败:", err)
		}
	}
	return false
}

// 已连接的 p2pSocks 在外部判断
func (r *P2pService) isNeedP2P(dstMac uint64) bool {
	if common.IsUniCast(dstMac) {
		if v, ok := r.hisP2PFailInfo.Load(dstMac); ok {
			p := v.(*model.PunchFailInfo)
			if p.FailedCount >= 10 || // 一方为 NatSymmetric 另一方为 NatSymmetric或NatPortCone 一般无法打洞
				uint32(time.Now().Sub(p.FailedTime).Seconds()) < config.AppConfig.P2pRetryInterval {
				return false
			}
		}

		if app.RuntimeService.FindPeer(dstMac) == nil {
			return false
		}

		if _, ok := r.p2pTrySocks.Load(dstMac); ok {
			return false
		}
		return true
	} else {
		return false
	}
}

func (r *P2pService) StartPunch(dstMac uint64, trigger bool) error {
	if v, ok := r.punchSocks.Load(dstMac); ok {
		p := v.(*model.PunchSockContext)
		if time.Now().Unix()-p.StartTime < r.p2pFailedTime {
			if trigger {
				return r.SendP2PTrigger(dstMac) // 还存在sock 则继续上报
			} else {
				return nil
			}
		} else {
			p.Bootstrap.Stop()
			r.punchSocks.Delete(dstMac)
			return errors.New("交互Sock已失效")
		}
	}
	if _, err := r.newPunchSock(dstMac); err == nil {
		return nil
	} else {
		return err
	}
}

func (r *P2pService) newPunchSock(dstMac uint64) (*protocol.Sock, error) {
	// 创建Bootstrap
	ctx := &model.PunchSockContext{PeerMac: dstMac, StartTime: time.Now().Unix(), Bootstrap: netty.NewBootstrap()}
	r.punchSocks.Store(dstMac, ctx)

	// 子连接的流水线配置
	var initializer = func(channel netty.Channel) {
		channel.Pipeline().
			AddLast(format.ProtobufCodec(1, uint32(config.AppConfig.MaxPacketSize))).
			AddLast(NewP2pHandler(ctx))
	}
	ctx.Bootstrap.ClientInitializer(initializer)
	url := fmt.Sprint("//", config.AppConfig.ServerIp, ":", config.AppConfig.ServerPort)
	lAddr := &net.UDPAddr{IP: net.IPv4zero, Port: 0}

	ch, err := ctx.Bootstrap.Transport(udp.New()).Connect(url, nil, transport.WithLocalAddr(lAddr))
	if err != nil {
		r.saveFailPunchInfo(dstMac)
		return nil, errors.New(fmt.Sprintf("P2P交互Sock连接失败:%s", err.Error()))
	}
	if ip, port, e := common.ExtractAddrFromStr(ch.LocalAddr()); e == nil {
		ctx.LocalSock = &protocol.Sock{Addr: ip, Port: port, Family: syscall.AF_INET}
		return ctx.LocalSock, nil
	} else {
		return nil, e
	}
}

func (r *P2pService) SendP2PTrigger(dstMac uint64) error {
	if v, ok := r.punchSocks.Load(dstMac); ok {
		p := v.(*model.PunchSockContext)
		if time.Now().Unix()-p.StartTime >= r.p2pFailedTime {
			p.Bootstrap.Stop()
			r.punchSocks.Delete(dstMac)
			return errors.New("无效的P2P交互Sock")
		}
		if p.Handler == nil {
			return errors.New("无效的P2P交互Sock")
		}
		triggerMsg := &protocol.MsgP2PTrigger{SrcMac: config.AppConfig.TapConfig.HwMac, DstMac: dstMac}
		msg := &protocol.MsgPeerFrame{MsgType: protocol.MsgType_Msg_P2PTrigger, PeerMac: config.AppConfig.TapConfig.HwMac}
		msg.MsgP2PTrigger = triggerMsg
		p.Handler.Write(msg)
		return nil
	} else {
		return errors.New("没有找到P2P交互Sock")
	}
}

func (r *P2pService) ProcessP2PTrigger(msg *protocol.MsgP2PTrigger) error {
	if msg == nil {
		return errors.New("P2P应答消息为空")
	}
	if common.IsUniCast(msg.SrcMac) {
		if _, ok := r.p2pTrySocks.Load(msg.SrcMac); ok {
			return errors.New("不需要启动P2P")
		}

		if _, ok := r.p2pSocks.Load(msg.SrcMac); ok {
			return errors.New("不需要启动P2P")
		}

	} else {
		return errors.New("P2P目标MAC无效")
	}
	return r.StartPunch(msg.SrcMac, true)
}

// 收到对端SOCK信息
func (r *P2pService) ProcessP2PAck(msgAck *protocol.MsgP2PAck) error {
	if msgAck == nil || msgAck.OtherExternSock == nil {
		return errors.New("无效的p2p应答消息")
	}
	if !msgAck.Valid {
		r.saveFailPunchInfo(msgAck.PeerMac)
		return errors.New("两端的NAT不能启动P2P")
	}
	v, ok := r.punchSocks.Load(msgAck.PeerMac)
	if !ok || v.(*model.PunchSockContext).LocalSock == nil {
		return errors.New("没有有效的P2P交互Sock")
	}
	punchSock := v.(*model.PunchSockContext)
	// 创建Bootstrap
	ctx := &model.PeerSockContext{PeerMac: msgAck.PeerMac, Bootstrap: netty.NewBootstrap(), Heartbeat: 5, Offline: 15}
	r.p2pTrySocks.Store(msgAck.PeerMac, ctx)
	// 子连接的流水线配置
	var initializer = func(channel netty.Channel) {
		channel.Pipeline().
			AddLast(netty.ReadIdleHandler(time.Duration(ctx.Heartbeat) * time.Second)).
			AddLast(format.ProtobufCodec(1, uint32(config.AppConfig.MaxPacketSize))).
			AddLast(NewDataHandler(ctx))
	}
	ctx.Bootstrap.ClientInitializer(initializer)
	url := fmt.Sprint("//", common.Uint32toIpV4(msgAck.OtherExternSock.Addr), ":", msgAck.OtherExternSock.Port)
	lAddr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", common.Uint32toIpV4(punchSock.LocalSock.Addr), punchSock.LocalSock.Port))

	ch, err := ctx.Bootstrap.Transport(udp.New()).Connect(url, nil, transport.WithLocalAddr(lAddr))
	if err != nil {
		r.p2pTrySocks.Delete(msgAck.PeerMac)
		return errors.New(fmt.Sprintf("p2p connect peer failed:%s", err.Error()))
	}
	if ip, port, e := common.ExtractAddrFromStr(ch.LocalAddr()); e == nil {
		ctx.LocalSock = &protocol.Sock{Addr: ip, Port: port, Family: syscall.AF_INET}
	}
	return nil
}

// 互发打洞消息
func (r *P2pService) SendP2PTry(dstMac uint64) error {
	sock, _ := r.p2pTrySocks.Load(dstMac)
	if sock != nil {
		ctx := sock.(*model.PeerSockContext)
		ctx.TryConnCount++
		ctx.LastTryConnTime = time.Now().Unix()
		if ctx.Handler != nil {
			msgFrame := &protocol.MsgDataFrame{SrcMac: config.AppConfig.TapConfig.HwMac, DstMac: ctx.PeerMac, MsgType: protocol.MsgType_Msg_P2PTry}
			ctx.Handler.Write(msgFrame)
			return nil
		}
	}
	r.p2pTrySocks.Delete(dstMac)
	return errors.New("P2P连接失败")
}

// 打洞成功
func (r *P2pService) P2PSuccess(dstMac uint64, sock *model.PeerSockContext) error {
	if sock == nil {
		return errors.New("无效的套接字信息")
	}

	if v, ok := r.p2pTrySocks.Load(dstMac); ok {
		ctx := v.(*model.PeerSockContext)
		r.p2pSocks.Store(dstMac, ctx)
		r.p2pTrySocks.Delete(dstMac)
		msgFrame := &protocol.MsgDataFrame{SrcMac: config.AppConfig.TapConfig.HwMac, DstMac: ctx.PeerMac, MsgType: protocol.MsgType_Msg_P2PTry}
		ctx.Handler.Write(msgFrame)
		peer := app.RuntimeService.FindPeer(dstMac)
		if peer != nil {
			app.Logger.Info("P2P连接成功:->", peer.PeerName)
		}
	} else {
		r.p2pSocks.Store(dstMac, sock)
	}
	app.WailsApp.UpdatePeers()
	return nil
}
func (r *P2pService) PeerStateChanged(mac uint64, _ bool) error {
	r.hisP2PFailInfo.Delete(mac)

	if v, ok := r.p2pTrySocks.Load(mac); ok && v != nil {
		p := v.(*model.PeerSockContext)
		p.Bootstrap.Stop()
	}
	r.p2pTrySocks.Delete(mac)

	if v, ok := r.p2pSocks.Load(mac); ok && v != nil {
		p := v.(*model.PeerSockContext)
		p.Bootstrap.Stop()
	}
	r.p2pSocks.Delete(mac)

	return nil
}

func (r *P2pService) IsP2P(dstMac uint64) bool {
	if _, ok := r.p2pSocks.Load(dstMac); ok {
		return true
	}
	return false
}

func (r *P2pService) DeleteFailedP2P(mac uint64) {
	r.hisP2PFailInfo.Delete(mac)
}

func (r *P2pService) ResetP2PSocks() {
	common.ClearMap(r.punchSocks)
	common.ClearMap(r.hisP2PFailInfo)

	r.p2pTrySocks.Range(func(key, value interface{}) bool {
		v := value.(*model.PeerSockContext)
		v.Bootstrap.Stop()
		return true
	})
	common.ClearMap(r.p2pTrySocks)

	r.p2pSocks.Range(func(key, value interface{}) bool {
		v := value.(*model.PeerSockContext)
		v.Bootstrap.Stop()
		return true
	})
	common.ClearMap(r.p2pSocks)
}

func (r *P2pService) saveFailPunchInfo(mac uint64) {
	if v, ok := r.hisP2PFailInfo.Load(mac); ok {
		p := v.(*model.PunchFailInfo)
		p.FailedCount++
		p.FailedTime = time.Now()
	} else {
		r.hisP2PFailInfo.Store(mac, &model.PunchFailInfo{DstMac: mac, FailedCount: 1, FailedTime: time.Now()})
	}
	if v, ok := r.punchSocks.Load(mac); ok {
		v.(*model.PunchSockContext).Bootstrap.Stop()
	}
	r.punchSocks.Delete(mac)
}
