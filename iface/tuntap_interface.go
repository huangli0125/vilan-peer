package iface

import "vilan/model"

type TunTapInterface interface {
	//Initialize() error
	Start() error // 启动读取tun tap数据
	Stop() error
	AddRoute(dst, mask, gw string) error
	DelRoute(dst, mask, gw string) error
	WriteData2TunTap(data []byte) (int, error) // 向tun tap 写入数据
	State() model.TunTapState
	TapName() string
}
