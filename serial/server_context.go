package serial

import (
	"errors"
	"fmt"
	"sync"
	"time"
	"vilan/app"
	"vilan/netty"
	"vilan/serial/protocol"
	serial2 "vilan/sys/serial"
)

type PortContext struct {
	Running bool
	Port    *serial2.Port
	Config  *serial2.Config
}

type ServerContext struct {
	portService *PortService
	handler     netty.HandlerContext
	portCtx     *sync.Map //map[string]*PortContext
	LastReceive int64
}

func NewServerContext(portService *PortService, handler netty.HandlerContext) *ServerContext {
	s := &ServerContext{portService: portService, handler: handler}
	s.LastReceive = time.Now().Unix()
	s.portCtx = &sync.Map{}
	return s
}

func (s *ServerContext) Start() error {
	return nil
}

func (s *ServerContext) Stop() error {
	keys := make([]string, 0)
	s.portCtx.Range(func(key, value interface{}) bool {
		ctx := value.(*PortContext)
		ctx.Running = false
		_ = ctx.Port.Close()
		stateMsg := &message.MsgState{Name: key.(string), Close: true}
		serMsg := &message.MsgServerFrame{MsgType: message.MsgType_Msg_State, SrcId: s.portService.localId, MsgState: stateMsg}
		_ = s.write2Net(serMsg)
		keys = append(keys, key.(string))
		return true
	})
	for _, key := range keys {
		s.portCtx.Delete(key)
	}
	if s.handler != nil {
		s.handler.Close(nil)
	}
	return nil
}

func (s *ServerContext) processMsg(clientMsg *message.MsgClientFrame) error {
	if clientMsg == nil {
		return errors.New("msg is nil")
	}
	defer func() {
		if err := recover(); err != nil {
			app.Logger.Debug("报文处理异常：", err)
		}
	}()
	switch clientMsg.MsgType {
	case message.MsgType_Msg_Data:
		if clientMsg.MsgData == nil {
			return errors.New("data msg is nil")
		}
		msg := clientMsg.MsgData
		if v, ok := s.portCtx.Load(msg.Name); ok {
			ctx := v.(*PortContext)
			_, err := ctx.Port.Write(msg.Data)
			//_ = ctx.Port.Flush()
			if msg.NeedAck {
				dataAck := &message.MsgDataAck{Sn: msg.Sn, Name: msg.Name, Result: err == nil}
				serMsg := &message.MsgServerFrame{MsgType: message.MsgType_Msg_DataAck, SrcId: s.portService.localId, MsgDataAck: dataAck}
				return s.write2Net(serMsg)
			}
		}
		return nil
	case message.MsgType_Msg_OpenPort:
		if clientMsg.MsgOpenCom == nil {
			return errors.New("port open msg is nil")
		}
		msg := clientMsg.MsgOpenCom
		openAck := &message.MsgOpenPortAck{Name: msg.Name, Result: false}
		serMsg := &message.MsgServerFrame{MsgType: message.MsgType_Msg_OpenPortAck, SrcId: s.portService.localId, MsgOpenPortAck: openAck}

		conf := &serial2.Config{Name: msg.Name, Baud: 9600, Size: 8, Parity: serial2.ParityNone, StopBits: serial2.Stop1, ReadTimeout: 25}
		needSave := true
		if msg.Config == nil {
			if c, e := s.portService.GetLocalPortConfig(msg.Name); e == nil && c != nil {
				conf = c
				needSave = false
			}
		} else {
			conf = &serial2.Config{
				Name: msg.Name, Baud: int(msg.Config.Baud), Size: byte(msg.Config.DataBit),
				Parity: serial2.Parity(msg.Config.DataParity), StopBits: serial2.StopBits(msg.Config.StopBit),
			}
		}
		if v, ok := s.portCtx.Load(msg.Name); ok && v != nil {
			p := v.(*PortContext)
			if p != nil && p.Port != nil {
				_ = p.Port.Close()
			}
		}
		s.portCtx.Delete(msg.Name)

		if port, err := serial2.OpenPort(conf); err != nil {
			openAck.Result = false
			openAck.Tip = "串口打开失败"
			_ = s.write2Net(serMsg)
			app.Logger.Warn(fmt.Sprintf("串口%s打开失败:%s", conf.Name, err))
		} else {
			pc := &PortContext{Port: port, Config: conf}
			s.portCtx.Store(msg.Name, pc)
			openAck.Result = true
			openAck.Tip = "串口打开成功"
			openAck.Config = &message.MsgConfig{Name: conf.Name, Baud: uint32(conf.Baud),
				DataBit: uint32(conf.Size), DataParity: uint32(conf.Parity), StopBit: uint32(conf.StopBits)}
			_ = s.write2Net(serMsg)
			app.Logger.Warn(fmt.Sprintf("串口%s打开成功!", conf.Name))
			go s.readSerial(pc)
		}
		if needSave {
			_ = serial2.SaveConfig(conf)
		}
		return nil
	case message.MsgType_Msg_ClosePort:
		if clientMsg.MsgCloseCom == nil {
			return errors.New("port close msg is nil")
		}
		msg := clientMsg.MsgCloseCom
		closeAck := &message.MsgClosePortAck{Name: msg.Name, Result: true}
		serMsg := &message.MsgServerFrame{MsgType: message.MsgType_Msg_ClosePortAck, SrcId: s.portService.localId, MsgClosePortAck: closeAck}

		if v, ok := s.portCtx.Load(msg.Name); ok {
			ctx := v.(*PortContext)
			_ = ctx.Port.Close()
			ctx.Running = false
		}
		s.portCtx.Delete(msg.Name)

		return s.write2Net(serMsg)
	case message.MsgType_Msg_GetConfig:
		if clientMsg.MsgGetConfig == nil {
			return errors.New("port config msg is nil")
		}
		var conf *serial2.Config = nil
		msg := clientMsg.MsgGetConfig
		if v, ok := s.portCtx.Load(msg.Name); ok {
			ctx := v.(*PortContext)
			conf = ctx.Config
		}
		if conf == nil {
			if c, err := s.portService.GetLocalPortConfig(msg.Name); err == nil {
				conf = c
			}
		}
		ack := &message.MsgGetConfigAck{Name: msg.Name, Result: conf != nil}
		ack.Config = &message.MsgConfig{Name: conf.Name, Baud: uint32(conf.Baud),
			DataBit: uint32(conf.Size), DataParity: uint32(conf.Parity), StopBit: uint32(conf.StopBits)}
		if conf == nil {
			ack.Tip = "参数获取失败"
		} else {
			ack.Tip = "参数获取成功"
		}
		serMsg := &message.MsgServerFrame{MsgType: message.MsgType_Msg_GetConfigAck, SrcId: s.portService.localId, MsgGetConfigAck: ack}
		return s.write2Net(serMsg)
	case message.MsgType_Msg_GetPortList:
		ack := &message.MsgGetPortListAck{}
		if comList, err := s.portService.GetLocalPorts(); err != nil {
			ack.Result = false
			ack.Tip = "串口列表获取失败"
		} else {
			ack.Result = true
			ack.Tip = "串口列表获取成功"
			if comList == nil {
				comList = make([]string, 0)
			}
			ack.PortList = comList
		}
		serMsg := &message.MsgServerFrame{MsgType: message.MsgType_Msg_GetPortListAck, SrcId: s.portService.localId, MsgPortListAck: ack}
		return s.write2Net(serMsg)
	default:
		return errors.New("")

	}
}

func (s *ServerContext) readSerial(ctx *PortContext) {
	if ctx == nil {
		return
	}
	ctx.Running = true
	defer func() {
		recover()
		if ctx.Running { // 异常
			_ = ctx.Port.Close()
			stateMsg := &message.MsgState{Name: ctx.Config.Name, Close: true}
			serMsg := &message.MsgServerFrame{MsgType: message.MsgType_Msg_State, SrcId: s.portService.localId, MsgState: stateMsg}
			_ = s.write2Net(serMsg)
		}
	}()
	buf := make([]byte, 2000)
	sn := uint32(0)
	port := ctx.Port
	for ctx.Running {
		if n, err := port.Read(buf); err != nil || n <= 0 {
			time.Sleep(2 * time.Second)
			continue
		} else {
			sn++
			dataMsg := &message.MsgData{Sn: sn, Name: ctx.Config.Name, NeedAck: false, Data: buf[:n]}
			serMsg := &message.MsgServerFrame{MsgType: message.MsgType_Msg_Data, SrcId: s.portService.localId, MsgData: dataMsg}
			_ = s.write2Net(serMsg)
		}
	}
}

func (s *ServerContext) write2Net(msg *message.MsgServerFrame) (err error) {
	if msg == nil {
		return errors.New("msg is nil")
	}
	defer func() {
		recover()
	}()
	err = errors.New("fail to write msg")
	s.handler.Write(msg)
	return nil
}
