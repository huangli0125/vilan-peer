package app

import (
	"vilan/common"
	"vilan/iface"
)

var Version = "2.001.20230701"

var Logger *common.Logger // 全局日志

var RuntimeService iface.RuntimeInterface
var P2pService iface.P2pInterface
var TunTapService iface.TunTapInterface
var WailsApp iface.WailsInterface

var SerialService iface.SerialInterface
