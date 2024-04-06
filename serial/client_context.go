package serial

import (
	"errors"
	"fmt"
	"sync"
	"time"
	"vilan/app"
	"vilan/netty"
	"vilan/netty/transport/tcp"
	"vilan/serial/protocol"
	serial2 "vilan/sys/serial"
)

type ResultType string

const (
	ResultAuthAck   ResultType = "AuthAck"
	ResultOpenPort  ResultType = "OpenPort"
	ResultClosePort ResultType = "ClosePort"
	//ResultDataMsgAck ResultType = "DataMsgAck"
	ResultGetPortConfig ResultType = "GetPortConfig"
	ResultGetPortList   ResultType = "GetPortList"
)

type ClientContext struct {
	portService   *PortService
	url           string
	bootstrap     netty.Bootstrap
	clientChannel netty.Channel
	handler       netty.HandlerContext
	portList      []string  // 远端的串口列表
	portCtx       *sync.Map //map[string]*PortContext // config 中的name 为本地串口

	LastReceive int64
	cmdTimeout  time.Duration
	msgResult   map[string]chan map[string]interface{}
}

func NewClientContext(portService *PortService, url string) *ClientContext {
	c := &ClientContext{portService: portService, url: url, cmdTimeout: 1200 * time.Millisecond}
	c.msgResult = make(map[string]chan map[string]interface{})
	return c
}

func (c *ClientContext) Start() (err error) {
	c.portCtx = &sync.Map{}
	//////////////  网络连接 /////////////////////////
	var bootstrap = netty.NewBootstrap()
	// 设置子连接的流水线配置
	var clientPipelineInitializer = func(channel netty.Channel) {
		channel.Pipeline().
			AddLast(message.ProtobufCodec(0, 2048)).
			AddLast(netty.ReadIdleHandler(32 * time.Second)).
			AddLast(NewClientHandler(c.portService))
	}
	// TCP 参数
	tcpOptions := &tcp.Options{
		Timeout:         time.Second * 5,
		KeepAlive:       true,
		KeepAlivePeriod: time.Minute, // tcp ping 等待间隔
		Linger:          0,
		NoDelay:         true,
		//SockBuf:         8192,
	}
	bootstrap.ClientInitializer(clientPipelineInitializer).
		Transport(tcp.New())
	c.clientChannel, err = bootstrap.Connect(c.url, nil, tcp.WithOptions(tcpOptions))
	if err == nil {
		c.LastReceive = time.Now().Unix()
	}
	return err
}

func (c *ClientContext) Stop() error {
	// 发送断开指令
	clientMsg := &message.MsgClientFrame{MsgType: message.MsgType_Msg_UnAuth, SrcId: c.portService.localId}
	_ = c.write2Net(clientMsg)
	keys := make([]string, 0)
	c.portCtx.Range(func(key, value interface{}) bool {
		ctx := value.(*PortContext)
		ctx.Running = false
		_ = ctx.Port.Close()
		keys = append(keys, key.(string))
		return true
	})

	for _, key := range keys {
		c.portCtx.Delete(key)
	}
	time.Sleep(50 * time.Millisecond)
	if c.clientChannel != nil {
		_ = c.clientChannel.Close()
		c.clientChannel = nil
	}
	if c.handler != nil {
		c.handler.Close(nil)
	}
	if c.bootstrap != nil {
		c.bootstrap.Stop()
	}
	return nil
}

func (c *ClientContext) SetHandler(handler netty.HandlerContext) {
	c.handler = handler
}

func (c *ClientContext) processMsg(clientMsg *message.MsgServerFrame) error {
	if clientMsg == nil {
		return errors.New("msg is nil")
	}
	defer func() {
		recover()
	}()
	switch clientMsg.MsgType {
	case message.MsgType_Msg_Data:
		if clientMsg.MsgData == nil {
			return errors.New("data msg is nil")
		}
		msg := clientMsg.MsgData
		if v, ok := c.portCtx.Load(msg.Name); ok {
			ctx := v.(*PortContext)
			_, _ = ctx.Port.Write(msg.Data)
			//_ = ctx.Port.Flush() // 此处打开后  写入后马上来数据会在读协程报错
		}
		return nil
	case message.MsgType_Msg_DataAck:
		if clientMsg.MsgDataAck == nil {
			return errors.New("data ack msg is nil")
		}
		msg := clientMsg.MsgDataAck
		app.Logger.Debug(fmt.Sprintf("串口:<%s>,序列:%d,报文发送成功。", msg.Name, msg.Sn))
		break
	case message.MsgType_Msg_AuthAck:
		if clientMsg.MsgAuthAck == nil {
			if res, ok := c.msgResult[string(ResultAuthAck)]; ok {
				res <- map[string]interface{}{"result": false}
			}
			return errors.New("auth ack msg is nil")
		}
		if res, ok := c.msgResult[string(ResultAuthAck)]; ok {
			res <- map[string]interface{}{"result": true}
		}
		return nil
	case message.MsgType_Msg_OpenPortAck:
		if clientMsg.MsgOpenPortAck == nil {
			return errors.New("port open ack msg is nil")
		}
		msg := clientMsg.MsgOpenPortAck
		if res, ok := c.msgResult[msg.Name+string(ResultOpenPort)]; ok {
			res <- map[string]interface{}{"result": msg.Result, "config": msg.Config}
		}
		return nil
	case message.MsgType_Msg_ClosePortAck:
		if clientMsg.MsgClosePortAck == nil {
			return errors.New("port close ack msg is nil")
		}
		msg := clientMsg.MsgClosePortAck
		if res, ok := c.msgResult[msg.Name+string(ResultClosePort)]; ok {
			res <- map[string]interface{}{"result": msg.Result}
		}
		return nil
	case message.MsgType_Msg_GetConfigAck:
		if clientMsg.MsgGetConfigAck == nil {
			return errors.New("port config msg is nil")
		}
		msg := clientMsg.MsgGetConfigAck
		if res, ok := c.msgResult[msg.Name+string(ResultGetPortConfig)]; ok {
			if msg.Result && msg.Config != nil {
				conf := &serial2.Config{
					Name: msg.Name, Baud: int(msg.Config.Baud), Size: byte(msg.Config.DataBit),
					Parity: serial2.Parity(msg.Config.DataParity), StopBits: serial2.StopBits(msg.Config.StopBit),
				}
				res <- map[string]interface{}{"result": true, "config": conf, "tip": msg.Tip}
			} else {
				res <- map[string]interface{}{"result": false, "tip": msg.Tip}
			}
		}
		return nil
	case message.MsgType_Msg_GetPortListAck:
		if clientMsg.MsgPortListAck == nil {
			return errors.New("port list msg is nil")
		}
		msg := clientMsg.MsgPortListAck
		if res, ok := c.msgResult[string(ResultGetPortList)]; ok {
			c.portList = make([]string, 0)
			if msg.Result && msg.PortList != nil {
				c.portList = append(c.portList, msg.PortList...)
				res <- map[string]interface{}{"result": true, "list": c.portList, "tip": msg.Tip}
			} else {
				res <- map[string]interface{}{"result": false, "tip": msg.Tip}
			}
		}
		return nil
	default:
		return errors.New("")

	}
	return nil
}

func (c *ClientContext) WaitAuth() map[string]interface{} {
	res := make(chan map[string]interface{})
	c.msgResult[string(ResultAuthAck)] = res
	defer delete(c.msgResult, string(ResultAuthAck))
	select {
	case result := <-res:
		if result["result"] == true {
			return map[string]interface{}{"result": true, "tip": "注册成功"}
		} else {
			return map[string]interface{}{"result": false, "tip": "注册回复异常"}
		}
	case <-time.After(c.cmdTimeout):
		return map[string]interface{}{"result": false, "timeout": true, "tip": "注册回复超时"}
	}
}

// remote:远端串口;local 与 user 是本地的一对虚拟串口,config 为参数（可为空）
func (c *ClientContext) OpenPort(remote string, local string, user string, config *serial2.Config) map[string]interface{} {
	errTip := ""
	c.portCtx.Range(func(key, value any) bool {
		pCtx := value.(*PortContext)
		if key == remote {
			errTip = fmt.Sprintf("远端串口<%s>已与本地串口<%s>建立连接", remote, pCtx.Config.Name)
			return false
		} else if pCtx.Config.Name == local {
			errTip = fmt.Sprintf("本地串口<%s>已与远端串口<%s>建立连接", user, remote)
			return false
		}
		return true
	})
	if errTip != "" {
		return map[string]interface{}{"result": false, "tip": errTip}
	}
	openMsg := &message.MsgOpenPort{Name: remote}
	if config != nil {
		openMsg.Config = &message.MsgConfig{Name: remote, Baud: uint32(config.Baud), DataBit: uint32(config.Size),
			DataParity: uint32(config.Parity), StopBit: uint32(config.StopBits)}
	}
	clientMsg := &message.MsgClientFrame{MsgType: message.MsgType_Msg_OpenPort, SrcId: c.portService.localId, MsgOpenCom: openMsg}
	if c.write2Net(clientMsg) != nil {
		return map[string]interface{}{"result": false, "tip": "指令发送失败"}
	}
	res := make(chan map[string]interface{})
	c.msgResult[remote+string(ResultOpenPort)] = res
	defer delete(c.msgResult, remote+string(ResultOpenPort))
	select {
	case result := <-res:
		if result["result"] == true {
			remoteConfig := result["config"].(*message.MsgConfig)
			if remoteConfig == nil {
				return map[string]interface{}{"result": false, "tip": "远端串口打开失败:无法回传串口参数"}
			}
			if config == nil {
				config = &serial2.Config{Baud: int(remoteConfig.Baud), Size: byte(remoteConfig.DataBit),
					StopBits: serial2.StopBits(remoteConfig.StopBit), Parity: serial2.Parity(remoteConfig.DataParity), ReadTimeout: 0}
			}
			config.RemoteName = remote
			config.Name = local
			config.UserPortName = user
			port, err := serial2.OpenPort(config)
			if err != nil || port == nil {
				return map[string]interface{}{"result": false, "tip": "本地串口打开失败:" + err.Error()}
			}
			pCtx := &PortContext{Port: port, Config: config}
			c.portCtx.Store(remote, pCtx)
			go c.readSerial(remote, pCtx)
			return map[string]interface{}{"result": true, "tip": "远端串口打开成功"}
		} else {
			return map[string]interface{}{"result": false, "tip": "远端串口打开失败"}
		}
	case <-time.After(c.cmdTimeout):
		return map[string]interface{}{"result": false, "timeout": true, "tip": "远端串口打开超时,没有收到指令"}
	}
}

// 远端串口的名称
func (c *ClientContext) ClosePort(name string) map[string]interface{} {
	closeMsg := &message.MsgClosePort{Name: name}
	clientMsg := &message.MsgClientFrame{MsgType: message.MsgType_Msg_ClosePort, SrcId: c.portService.localId, MsgCloseCom: closeMsg}
	if c.write2Net(clientMsg) != nil {
		return map[string]interface{}{"result": false, "tip": "指令发送失败"}
	}
	res := make(chan map[string]interface{})
	c.msgResult[name+string(ResultClosePort)] = res
	defer delete(c.msgResult, name+string(ResultClosePort))
	if v, ok := c.portCtx.Load(name); ok {
		ctx := v.(*PortContext)
		_ = ctx.Port.Close()
		ctx.Running = false
	}
	c.portCtx.Delete(name)
	select {
	case result := <-res:
		if result["result"] == true {
			return map[string]interface{}{"result": true, "tip": "远端串口关闭成功"}
		} else {
			return map[string]interface{}{"result": false, "tip": "远端串口关闭失败"}
		}
	case <-time.After(c.cmdTimeout):
		return map[string]interface{}{"result": false, "timeout": true, "tip": "远端串口关闭超时,没有收到指令"}
	}
}

func (c *ClientContext) GetPortConfig(name string) map[string]interface{} {
	configMsg := &message.MsgGetConfig{Name: name}
	clientMsg := &message.MsgClientFrame{MsgType: message.MsgType_Msg_GetConfig, SrcId: c.portService.localId, MsgGetConfig: configMsg}
	if c.write2Net(clientMsg) != nil {
		return map[string]interface{}{"result": false, "tip": "指令发送失败"}
	}
	res := make(chan map[string]interface{})
	c.msgResult[name+string(ResultGetPortConfig)] = res
	defer delete(c.msgResult, name+string(ResultGetPortConfig))
	select {
	case result := <-res:
		if result["result"] == true && result["config"] != nil {
			//app.Logger.Info(result["config"])
			return map[string]interface{}{"result": true, "config": result["config"], "tip": "远端串口参数获取成功"}
		} else {
			return map[string]interface{}{"result": false, "tip": "远端串口参数获取失败"}
		}
	case <-time.After(c.cmdTimeout):
		return map[string]interface{}{"result": false, "timeout": true, "tip": "远端串口参数获取超时,没有收到指令"}
	}
}
func (c *ClientContext) SetPortConfig(remote string, config *serial2.Config) map[string]interface{} {
	if v, ok := c.portCtx.Load(remote); ok {
		ctx := v.(*PortContext)
		return map[string]interface{}{"result": false, "tip": fmt.Sprintf("远端串口<%s>已与本地串口<%s>建立连接", remote, ctx.Config.Name)}
	}
	openMsg := &message.MsgOpenPort{Name: remote}
	if config != nil {
		openMsg.Config = &message.MsgConfig{Name: remote, Baud: uint32(config.Baud), DataBit: uint32(config.Size),
			DataParity: uint32(config.Parity), StopBit: uint32(config.StopBits)}
	}
	clientMsg := &message.MsgClientFrame{MsgType: message.MsgType_Msg_OpenPort, SrcId: c.portService.localId, MsgOpenCom: openMsg}
	if c.write2Net(clientMsg) != nil {
		return map[string]interface{}{"result": false, "tip": "指令发送失败"}
	}
	res := make(chan map[string]interface{})
	c.msgResult[remote+string(ResultOpenPort)] = res
	defer delete(c.msgResult, remote+string(ResultOpenPort))
	select {
	case result := <-res:
		if result["result"] == true {
			remoteConfig := result["config"].(*message.MsgConfig)
			if remoteConfig == nil {
				return map[string]interface{}{"result": false, "tip": "远端串口打开失败:无法回传串口参数"}
			}
			if config == nil {
				config = &serial2.Config{Baud: int(remoteConfig.Baud), Size: byte(remoteConfig.DataBit),
					StopBits: serial2.StopBits(remoteConfig.StopBit), Parity: serial2.Parity(remoteConfig.DataParity), ReadTimeout: 0}
			}
			return map[string]interface{}{"result": true, "tip": "远端串口参数设置成功"}
		} else {
			return map[string]interface{}{"result": false, "tip": "远端串口参数设置失败"}
		}
	case <-time.After(c.cmdTimeout):
		return map[string]interface{}{"result": false, "timeout": true, "tip": "远端串口参数设置超时,没有收到指令"}
	}
}

func (c *ClientContext) GetPortList() map[string]interface{} {
	clientMsg := &message.MsgClientFrame{MsgType: message.MsgType_Msg_GetPortList, SrcId: c.portService.localId}
	if c.write2Net(clientMsg) != nil {
		return map[string]interface{}{"result": false, "tip": "指令发送失败"}
	}
	res := make(chan map[string]interface{})
	c.msgResult[string(ResultGetPortList)] = res
	defer delete(c.msgResult, string(ResultGetPortList))
	select {
	case result := <-res:
		if result["result"] == true {
			return map[string]interface{}{"result": true, "list": result["list"], "tip": "远端串口列表获取成功"}
		} else {
			return map[string]interface{}{"result": false, "tip": "远端串口列表获取失败"}
		}
	case <-time.After(c.cmdTimeout):
		return map[string]interface{}{"result": false, "timeout": true, "tip": "远端串口列表获取超时,没有收到指令"}
	}
}
func (c *ClientContext) GetConnectedPorts() map[string]interface{} {
	list := make([]*serial2.Config, 0)
	c.portCtx.Range(func(key, value interface{}) bool {
		p := value.(*PortContext)
		list = append(list, p.Config)
		return true
	})
	return map[string]interface{}{"result": true, "list": list}
}

func (c *ClientContext) readSerial(name string, ctx *PortContext) {
	if ctx == nil {
		return
	}
	ctx.Running = true
	defer func() {
		recover()
	}()
	buf := make([]byte, 2000)
	sn := uint32(0)
	port := ctx.Port
	for ctx.Running {
		if n, err := port.Read(buf); err != nil || n <= 0 {
			time.Sleep(10 * time.Millisecond)
			continue
		} else {
			sn++
			dataMsg := &message.MsgData{Sn: sn, Name: name, NeedAck: false, Data: buf[:n]}
			serMsg := &message.MsgClientFrame{MsgType: message.MsgType_Msg_Data, SrcId: c.portService.localId, MsgData: dataMsg}
			_ = c.write2Net(serMsg)
		}
	}
}

func (c *ClientContext) write2Net(msg *message.MsgClientFrame) (err error) {
	if msg == nil {
		return errors.New("msg is nil")
	}
	if c.handler == nil {
		return errors.New("handler is nil")
	}
	defer func() {
		recover()
	}()
	err = errors.New("fail to write msg")
	c.handler.Write(msg)
	return nil
}
