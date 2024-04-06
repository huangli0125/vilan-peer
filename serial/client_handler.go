package serial

import (
	"errors"
	"vilan/app"
	"vilan/netty"
	"vilan/netty/codec"
	"vilan/serial/protocol"
)

type ClientHandler struct {
	server *PortService
	auth   bool
}

func NewClientHandler(server *PortService) codec.Codec {
	return &ClientHandler{server: server, auth: false}
}

func (s *ClientHandler) CodecName() string {
	return "client-codec"
}

// 通信开始，设备注册
func (s *ClientHandler) HandleActive(ctx netty.ActiveContext) {
	msgAuth := &message.MsgAuth{ClientId: s.server.localId}
	serMsg := &message.MsgClientFrame{MsgType: message.MsgType_Msg_Auth, SrcId: s.server.localId, MsgAuth: msgAuth}
	ctx.Write(serMsg)
}

// 信息处理
func (s *ClientHandler) HandleRead(ctx netty.InboundContext, msg netty.Message) {
	if msg == nil {
		return
	}
	switch v := msg.(type) {
	case *message.MsgServerFrame:
		_ = s.processMsg(v, ctx)
	default:
		return
	}
}
func (s *ClientHandler) processMsg(msg *message.MsgServerFrame, ctx netty.InboundContext) (err error) {
	if msg == nil {
		return errors.New("msg is nil")
	}
	switch msg.MsgType {
	case message.MsgType_Msg_AuthAck:
		s.auth = true
		s.server.ProcessServerMessage(msg.SrcId, msg, ctx)
	default:
		if !s.auth {
			return errors.New("client is not auth")
		}
		s.server.ProcessServerMessage(msg.SrcId, msg, ctx)
	}
	return nil
}

func (s *ClientHandler) HandleWrite(_ netty.OutboundContext, _ netty.Message) {

}

// 异常信息处理
func (s *ClientHandler) HandleException(ctx netty.ExceptionContext, ex netty.Exception) {
	app.Logger.Debug("peer exception: ", ex.Error())
	ctx.HandleException(ex)
}

// 设备离线
func (s *ClientHandler) HandleInactive(ctx netty.InactiveContext, ex netty.Exception) {
	app.Logger.Debug("peer:", "->", "inactive:", ctx.Channel().RemoteAddr(), ex)
	// 连接断开了，默认处理是关闭连接
	ctx.HandleInactive(ex)
}

// 超时信息处理
func (s *ClientHandler) HandleEvent(ctx netty.EventContext, event netty.Event) {
	switch event.(type) {
	case netty.ReadIdleEvent:
		if s.auth {
			pingMsg := &message.MsgClientFrame{MsgType: message.MsgType_Msg_Ping, SrcId: s.server.localId}
			ctx.Write(pingMsg)
		}
	case netty.WriteIdleEvent:

	default:
		app.Logger.Debug("未知超时！")
	}
}
