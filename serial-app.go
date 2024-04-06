package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"vilan/app"
	serial0 "vilan/serial"
	serial2 "vilan/sys/serial"
)

type SerialApp struct {
}

func NewSerialApp() *SerialApp {
	return &SerialApp{}
}
func (s *SerialApp) ConnectRemote(idStr string) map[string]interface{} {
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		app.Logger.Debug("ID转换错误:", idStr, err)
		return map[string]interface{}{"result": false}
	}
	return app.SerialService.ConnectRemote(id)
}

func (s *SerialApp) DisConnectRemote(idStr string) map[string]interface{} {
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return map[string]interface{}{"result": false}
	}
	return app.SerialService.DisConnectRemote(id)
}

// remote 远端串口 local、user 本地一对虚拟串口  config 串口参数
func (s *SerialApp) OpenPort(idStr string, remote string, local string, user string, config string) map[string]interface{} {
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return map[string]interface{}{"result": false, "tip": "串口打开失败,ID转换失败"}
	}
	if config == "" {
		return app.SerialService.OpenPort(id, remote, local, user, nil)
	}
	conf := &serial2.Config{}
	if err := json.Unmarshal([]byte(config), conf); err != nil {
		return map[string]interface{}{"result": false, "tip": "串口打开失败,无效的串口参数"}
	}
	return app.SerialService.OpenPort(id, remote, local, user, conf)
}

func (s *SerialApp) ClosePort(idStr string, name string) map[string]interface{} {
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return map[string]interface{}{"result": false, "tip": "串口关闭失败,ID转换失败"}
	}
	return app.SerialService.ClosePort(id, name)
}
func (s *SerialApp) GetPortConfig(idStr string, name string) map[string]interface{} {
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		app.Logger.Debug("ID转换错误:", idStr, err)
		return nil
	}
	return app.SerialService.GetPortConfig(id, name)
}

func (s *SerialApp) SetPortConfig(idStr string, remote string, config string) map[string]interface{} {
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		app.Logger.Debug("ID转换错误:", idStr, err)
		return nil
	}
	conf := &serial2.Config{}
	if err := json.Unmarshal([]byte(config), conf); err != nil {
		return map[string]interface{}{"result": false, "tip": "串口设置失败,无效的串口参数"}
	}
	return app.SerialService.SetPortConfig(id, remote, conf)
}

func (s *SerialApp) GetConnectedPorts(idStr string) map[string]interface{} {
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return map[string]interface{}{"result": false, "tip": "已连接串口获取失败,ID转换失败"}
	}
	return app.SerialService.GetConnectedPorts(id)
}
func (s *SerialApp) GetRemotePortList(idStr string) map[string]interface{} {
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return map[string]interface{}{"result": false, "tip": "远端串口获取失败,ID转换失败"}
	}
	return app.SerialService.GetRemotePortList(id)
}

// virtual com

func (s *SerialApp) GetLocalPortList() map[string]interface{} {
	pairs, err := serial0.LoadComPairs()
	if err != nil {
		return map[string]interface{}{"result": false, "tip": fmt.Sprint("本地串口获取失败:", err)}
	}
	return map[string]interface{}{"result": true, "list": pairs, "tip": "本地串口获取成功"}
}

func (s *SerialApp) AddLocalPort(comA string, comB string) map[string]interface{} {
	pair, err := serial0.AddComPair(comA, comB)
	if err != nil {
		return map[string]interface{}{"result": false, "tip": fmt.Sprint("本地串口创建失败:", err)}
	}
	return map[string]interface{}{"result": true, "pair": pair, "tip": "本地串口创建成功"}
}

func (s *SerialApp) DelLocalPort(index uint) map[string]interface{} {
	err := serial0.DelComPair(index)
	if err != nil {
		return map[string]interface{}{"result": false, "tip": fmt.Sprint("本地串口删除失败:", err)}
	}
	return map[string]interface{}{"result": true, "tip": "本地串口删除成功"}
}
