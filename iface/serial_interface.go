package iface

import "vilan/sys/serial"

type SerialInterface interface {
	Start() error
	Stop()
	ConnectRemote(dstId uint64) map[string]interface{}
	DisConnectRemote(dstId uint64) map[string]interface{}
	OpenPort(dstId uint64, remote string, local string, user string, config *serial.Config) map[string]interface{}
	ClosePort(dstId uint64, name string) map[string]interface{}
	GetPortConfig(dstId uint64, name string) map[string]interface{}
	SetPortConfig(dstId uint64, remote string, config *serial.Config) map[string]interface{}
	GetConnectedPorts(dstId uint64) map[string]interface{}
	GetRemotePortList(dstId uint64) map[string]interface{}
	GetLocalPortList() map[string]interface{}
	PeerStateChanged(mac uint64, online bool) error
}
