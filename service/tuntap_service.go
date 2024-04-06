package service

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
	"vilan/app"
	"vilan/common"
	"vilan/config"
	"vilan/model"
	"vilan/tuntap"
)

type TunTapService struct {
	tapper  tuntap.TapDevice
	state   model.TunTapState
	readBuf []byte
	macBuf  []byte
}

func NewTunTapService() *TunTapService {
	return &TunTapService{tapper: nil, state: model.TunTapUnInit, macBuf: make([]byte, 8)}
}

func (t *TunTapService) createTap() (err error) {
	if config.AppConfig.TapConfig == nil {
		t.state = model.TunTapUnInit
		return errors.New("虚拟网卡没有有效配置")
	}
	if config.AppConfig.TapConfig.HwMac == 0 || config.AppConfig.TapConfig.IpAddr == 0 {
		t.state = model.TunTapUnInit
		return errors.New("虚拟网卡目前没有有效的IP或MAC地址")
	}
	t.tapper, err = tuntap.CreateTapDevice(config.AppConfig.TapConfig.Name)
	if err != nil {
		t.state = model.TunTapInitFailed
		return err
	} else if t.tapper == nil {
		t.state = model.TunTapInitFailed
		return errors.New("虚拟网卡初始化失败")
	}
	t.state = model.TunTapInitOk
	return nil
}

func (t *TunTapService) Start() error {
	defer func() {
		if e := recover(); e != nil {
			app.Logger.Error("网卡初始化异常:", e)
		}
	}()
	if t.state == model.TunTapRunning {
		return nil
	}
	if err := t.createTap(); err != nil {
		return err
	}
	if curMac, err := t.tapper.GetMac(); err != nil {
		return errors.New(fmt.Sprintf("虚拟网卡MAC地址获取失败:%s", err.Error()))
	} else {
		if curMac != config.AppConfig.TapConfig.HwMac {
			if e := t.tapper.SetMac(config.AppConfig.TapConfig.HwMac); e != nil {
				return errors.New(fmt.Sprintf("虚拟网卡MAC地址设置失败:%s", e.Error()))
			}
		}
	}
	mask := uint32(0xFFFFFFFF) << (32 - config.AppConfig.TapConfig.IpMask)
	if err := t.tapper.SetIpAddr(common.Uint32toIpV4(config.AppConfig.TapConfig.IpAddr), common.Uint32toIpV4(mask)); err != nil {
		return errors.New(fmt.Sprintf("虚拟网卡IP地址设置失败:%s", err.Error()))
	}
	_ = t.tapper.SetMtu(1350)
	_ = t.tapper.Up()
	t.readBuf = make([]byte, model.SizeMaxPacket)
	// 开启协程
	go t.readTap()
	return nil
}
func (t *TunTapService) Stop() error {
	t.state = model.TunTapStop
	if t.tapper != nil {
		//t.tapper.Down()
		err := t.tapper.Close()
		if err != nil {
			return err
		}
		t.tapper = nil
	}
	return nil
}
func (t *TunTapService) State() model.TunTapState {
	return t.state
}

func (t *TunTapService) readTap() {
	defer func() {
		if e := recover(); e != nil {
		}
		if t.tapper != nil {
			_ = t.tapper.Close()
		}
		t.state = model.TunTapStop
	}()
	t.state = model.TunTapRunning
	for t.state == model.TunTapRunning {
		n, err := t.tapper.Read(t.readBuf[:])
		if err != nil || n == 0 {
			if t.state != model.TunTapRunning {
				return
			}
			time.Sleep(100 * time.Millisecond)
			// 重启网卡
			_ = t.Stop()
			if e := t.createTap(); e == nil {
				_ = t.tapper.Up()
				t.state = model.TunTapRunning
			} else {
				return
			}
			continue
		}
		// 处理tun tap读取的数据
		t.tapData2Net(t.readBuf[:n])
	}
}

func (t *TunTapService) tapData2Net(data []byte) {
	dataLen := len(data)
	// dst mac 6
	// src mac 6
	// frame type 2
	if dataLen < model.SizeEthFrame { //model.SizeMac *2 + 2
		return
	}
	// 取出数据 封装为MsgPacket  找到出口sock(server或者已建立的p2p)  发送数据
	copy(t.macBuf[:], data[:6])
	dstMac := binary.LittleEndian.Uint64(t.macBuf)
	_ = app.RuntimeService.PostTunTapData(dstMac, data[:]) // 转发tap数据到相关socket
}

func (t *TunTapService) WriteData2TunTap(data []byte) (int, error) {
	if t.state != model.TunTapRunning {
		return 0, errors.New("虚拟网卡服务未运行")
	}
	if t.tapper == nil {
		return 0, errors.New("虚拟网络不能正常启动")
	}
	return t.tapper.Write(data[:])
}
func (t *TunTapService) TapName() string {
	if t.tapper != nil {
		return t.tapper.Name()
	}
	return ""
}
func (t *TunTapService) AddRoute(dst, mask, gw string) error {
	_, err := tuntap.AddRoute(dst, mask, gw)
	return err
}

func (t *TunTapService) DelRoute(dst, mask, gw string) error {
	_, err := tuntap.DelRoute(dst, mask, gw)
	return err
}
