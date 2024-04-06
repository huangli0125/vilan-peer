package service

import (
	"time"
	"vilan/app"
	"vilan/config"
	"vilan/model"
	"vilan/netty"
	"vilan/netty/codec"
	"vilan/protocol"
)

type DataHandler struct {
	sockContext *model.PeerSockContext
}

func NewDataHandler(ctx *model.PeerSockContext) codec.Codec {
	return &DataHandler{sockContext: ctx}
}

func (p *DataHandler) CodecName() string {
	return "peer-codec"
}

// 通信开始，设备注册
func (p *DataHandler) HandleActive(ctx netty.ActiveContext) {
	app.Logger.Debug("peer connection:", "->", "active:", ctx.Channel().RemoteAddr())
	p.sockContext.Connected = true
	p.sockContext.Handler = ctx
	msgFrame := &protocol.MsgDataFrame{SrcMac: config.AppConfig.TapConfig.HwMac, DstMac: p.sockContext.PeerMac, MsgType: protocol.MsgType_Msg_Ping}
	ctx.Write(msgFrame)
}

// 信息处理
func (p *DataHandler) HandleRead(ctx netty.InboundContext, msg netty.Message) {
	if msg == nil {
		return
	}
	p.sockContext.Handler = ctx
	switch v := msg.(type) {
	case *protocol.MsgDataFrame:
		p.sockContext.LastReceive = time.Now().Unix()
		p.sockContext.PingTryCount = 0
		p.processMsg(v, ctx)
	default:
		app.Logger.Debug("PeerHandler未解析类型消息:", v)
		return
	}
}
func (p *DataHandler) HandleWrite(_ netty.OutboundContext, _ netty.Message) {

}
func (p *DataHandler) processMsg(msg *protocol.MsgDataFrame, ctx netty.InboundContext) {
	if msg == nil {
		return
	}
	switch msg.MsgType {
	case protocol.MsgType_Msg_Packet:
		if _, err := app.TunTapService.WriteData2TunTap(msg.Data[:]); err != nil { // todo 数据解密
			app.Logger.Error("data write to tun tap failed:", err)
		}
		break
	case protocol.MsgType_Msg_P2PTry:
		if err := app.P2pService.P2PSuccess(msg.SrcMac, p.sockContext); err != nil {
			app.Logger.Error("p2p failed:", err)
		}
		break
	case protocol.MsgType_Msg_Ping:
		msgFrame := &protocol.MsgDataFrame{SrcMac: config.AppConfig.TapConfig.HwMac, DstMac: msg.SrcMac, MsgType: protocol.MsgType_Msg_Pong}
		ctx.Write(msgFrame)
		break
	case protocol.MsgType_Msg_Pong:
		break
	}
}

// 异常信息处理
func (p *DataHandler) HandleException(ctx netty.ExceptionContext, ex netty.Exception) {
	app.Logger.Error("peer exception: ", ex.Error())
	//stackBuffer := bytes.NewBuffer(nil)
	//ex.PrintStackTrace(stackBuffer)
	ctx.HandleException(ex)
}

// 设备离线
func (p *DataHandler) HandleInactive(ctx netty.InactiveContext, ex netty.Exception) {
	app.Logger.Debug("peer:", "->", "inactive:", ctx.Channel().RemoteAddr(), ex)
	// 连接断开了，默认处理是关闭连接
	ctx.HandleInactive(ex)
}

// 超时信息处理
func (p *DataHandler) HandleEvent(_ netty.EventContext, event netty.Event) {
	switch event.(type) {
	case netty.ReadIdleEvent:
		msgFrame := &protocol.MsgDataFrame{SrcMac: config.AppConfig.TapConfig.HwMac, DstMac: p.sockContext.PeerMac, MsgType: protocol.MsgType_Msg_Ping}
		if p.sockContext.Handler != nil {
			p.sockContext.PingTryCount++
			p.sockContext.Handler.Write(msgFrame)
		}
	case netty.WriteIdleEvent:

	default:
		app.Logger.Debug("未知超时！")
	}
}
