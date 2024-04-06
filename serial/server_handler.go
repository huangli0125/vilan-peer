package serial

import (
	"errors"
	"vilan/app"
	"vilan/netty"
	"vilan/netty/codec"
	"vilan/serial/protocol"
)

type ServerHandler struct {
	server *PortService
	auth   bool
}

func NewServerHandler(server *PortService) codec.Codec {
	return &ServerHandler{server: server, auth: false}
}

func (s *ServerHandler) CodecName() string {
	return "server-codec"
}

// 通信开始，设备注册
func (s *ServerHandler) HandleActive(_ netty.ActiveContext) {
	//app.Logger.Info("successfully connect to server:", ctx.Channel().RemoteAddr())

}

// 信息处理
func (s *ServerHandler) HandleRead(ctx netty.InboundContext, msg netty.Message) {
	if msg == nil {
		return
	}
	switch v := msg.(type) {
	case *message.MsgClientFrame:
		_ = s.processMsg(v, ctx)
	default:
		return
	}
}

func (s *ServerHandler) processMsg(msg *message.MsgClientFrame, ctx netty.InboundContext) (err error) {
	if msg == nil {
		return errors.New("msg is nil")
	}
	switch msg.MsgType {
	case message.MsgType_Msg_Auth:
		s.auth = true
		s.server.ProcessClientMessage(msg.SrcId, msg, ctx)
	default:
		if !s.auth {
			return errors.New("client is not auth")
		}
		s.server.ProcessClientMessage(msg.SrcId, msg, ctx)
	}
	return nil
}

func (s *ServerHandler) HandleWrite(_ netty.OutboundContext, _ netty.Message) {

}

// 异常信息处理
func (s *ServerHandler) HandleException(ctx netty.ExceptionContext, ex netty.Exception) {
	app.Logger.Error("client exception: ", ex.Error())
	ctx.HandleException(ex)
}

// 设备离线
func (s *ServerHandler) HandleInactive(ctx netty.InactiveContext, ex netty.Exception) {
	app.Logger.Debug("client:", "->", "inactive:", ctx.Channel().RemoteAddr(), ex)
	// 连接断开了，默认处理是关闭连接
	ctx.HandleInactive(ex)

}

// 超时信息处理
func (s *ServerHandler) HandleEvent(ctx netty.EventContext, event netty.Event) {
	switch event.(type) {
	case netty.ReadIdleEvent:
		if s.auth {
			pingMsg := &message.MsgServerFrame{MsgType: message.MsgType_Msg_Ping, SrcId: s.server.localId}
			ctx.Write(pingMsg)
		}
	case netty.WriteIdleEvent:

	default:
		app.Logger.Debug("未知超时！")
	}
}
