package service

import (
	"errors"
	"time"
	"vilan/app"
	"vilan/model"
	"vilan/netty"
	"vilan/netty/codec"
	"vilan/protocol"
)

type MessageHandler struct {
	serverContext *model.ServerSockContext
	p2pContext    *model.PunchSockContext // 打洞时使用
}

func NewServerHandler(ctx *model.ServerSockContext) codec.Codec {
	return &MessageHandler{serverContext: ctx}
}
func NewP2pHandler(ctx *model.PunchSockContext) codec.Codec {
	return &MessageHandler{p2pContext: ctx}
}

func (s *MessageHandler) CodecName() string {
	return "server-codec"
}

// 通信开始，设备注册
func (s *MessageHandler) HandleActive(ctx netty.ActiveContext) {
	//app.Logger.Info("successfully connect to server:", ctx.Channel().RemoteAddr())
	if s.serverContext != nil { // msg transfer
		s.serverContext.Handler = ctx
		s.serverContext.Connected = true
		s.serverContext.LastReceive = time.Now().Unix()
		app.RuntimeService.SetPeerState(model.StateUnAck)
		_ = app.RuntimeService.SendAuthRequest()
	} else if s.p2pContext != nil { // p2p
		s.p2pContext.Handler = ctx
		s.p2pContext.Connected = true
		s.p2pContext.LastReceive = time.Now().Unix()
		_ = app.P2pService.SendP2PTrigger(s.p2pContext.PeerMac)
	}
}

// 信息处理
func (s *MessageHandler) HandleRead(ctx netty.InboundContext, msg netty.Message) {
	if msg == nil {
		return
	}
	if s.serverContext != nil {
		s.serverContext.LastReceive = time.Now().Unix()
		s.serverContext.Handler = ctx
	}
	if s.p2pContext != nil {
		s.p2pContext.LastReceive = time.Now().Unix()
		s.p2pContext.Handler = ctx
	}
	switch v := msg.(type) {
	case *protocol.MsgDataFrame:
		if n, e := app.TunTapService.WriteData2TunTap(v.Data[:]); e != nil || n != len(v.Data[:]) {
			//app.Logger.Error("Tap报文写入错误:", e)
		}
	case *protocol.MsgServerFrame:
		_ = s.processMsg(v, ctx)
	default:
		return
	}
}
func (s *MessageHandler) HandleWrite(_ netty.OutboundContext, _ netty.Message) {

}
func (s *MessageHandler) processMsg(msg *protocol.MsgServerFrame, _ netty.InboundContext) (err error) {
	if msg == nil {
		return errors.New("msg is nil")
	}
	switch msg.MsgType {
	case protocol.MsgType_Msg_AuthAck:
		err = app.RuntimeService.ProcessAuthResponse(msg.MsgAuthAck)
		if err != nil {
			app.Logger.Error(err)
		}
		break
	case protocol.MsgType_Msg_P2PTrigger:
		err = app.P2pService.ProcessP2PTrigger(msg.MsgP2PTrigger)
		break
	case protocol.MsgType_Msg_P2PAck:
		err = app.P2pService.ProcessP2PAck(msg.MsgP2PAck)
		break
	case protocol.MsgType_Msg_StateChanged:
		err = app.RuntimeService.ProcessPeerStateChanged(msg.MsgState)
		break
	case protocol.MsgType_Msg_Pong:
		err = app.RuntimeService.ProcessPong(msg.MsgPong)
		break
	case protocol.MsgType_Msg_GroupPeersResponse:
		err = app.RuntimeService.ProcessGroupPeersResponse(msg.MsgGroupPeerAck)
		break
	case protocol.MsgType_Msg_PeerLinksRequest:
		err = app.RuntimeService.ProcessPeerLinksRequest(msg.MsgLinks)
		break
	case protocol.MsgType_Msg_PeerLinksResponse:
		err = app.RuntimeService.ProcessPeerLinksResponse(msg.MsgLinksAck)
		break
	case protocol.MsgType_Msg_Config:
		err = app.RuntimeService.ProcessConfig(msg.MsgConfig)
		break
	case protocol.MsgType_Msg_ConfigAck:
		err = app.RuntimeService.ProcessConfigAck(msg.MsgConfigAck)
		break
	}
	return err
}

// 异常信息处理
func (s *MessageHandler) HandleException(ctx netty.ExceptionContext, ex netty.Exception) {
	app.Logger.Error("peer exception: ", ex.Error())
	ctx.HandleException(ex)
	if s.serverContext != nil {
		s.serverContext.Connected = false
	}
}

// 设备离线
func (s *MessageHandler) HandleInactive(ctx netty.InactiveContext, ex netty.Exception) {
	app.Logger.Debug("peer:", "->", "inactive:", ctx.Channel().RemoteAddr(), ex)
	// 连接断开了，默认处理是关闭连接
	ctx.HandleInactive(ex)
	if s.serverContext != nil {
		s.serverContext.Connected = false
	}
}

// 超时信息处理
func (s *MessageHandler) HandleEvent(_ netty.EventContext, event netty.Event) {
	switch event.(type) {
	case netty.ReadIdleEvent:
		if app.RuntimeService.PeerState() == model.StateOk {
			_ = app.RuntimeService.SendPing()
		}
	case netty.WriteIdleEvent:

	default:
		app.Logger.Debug("未知超时！")
	}
}
