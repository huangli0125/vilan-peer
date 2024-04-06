package model

import (
	"fmt"
	"strconv"
	"vilan/protocol"
)

type PeerInfo struct {
	PeerName    string `json:"peer_name,omitempty"`
	PeerMac     string `json:"peer_mac,omitempty"`
	DevType     string `json:"dev_type,omitempty"`
	NetAddr     string `json:"net_addr,omitempty"`
	InterAddr   string `json:"inter_addr,omitempty"`
	Online      bool   `json:"online"`
	LinkMode    uint32 `json:"link_mode"`
	LinkQuality uint32 `json:"link_quality"`
	ConnectType uint   `json:"connect_type"` // 0 转发 ,1 p2p
	TotalRxTx   string `json:"total_rx_tx"`
	P2PRxTx     string `json:"p2p_rx_tx"`
	TransRxTx   string `json:"trans_rx_tx"`
}

func ProtoToModel(info *protocol.PeerInfo) *PeerInfo {
	if info == nil {
		return nil
	}
	m := &PeerInfo{}
	m.PeerName = info.PeerName
	m.PeerMac = fmt.Sprintf("%d", info.PeerMac)
	m.DevType = info.DevType
	m.NetAddr = uint2IpV4(info.NetAddr) + "/" + strconv.Itoa(int(info.NetBitLen))
	m.InterAddr = uint2IpV4(info.InterAddr) + "/" + strconv.Itoa(int(info.InterNetBitLen))
	m.Online = info.Online
	m.ConnectType = 0
	m.LinkMode = info.LinkMode
	m.LinkQuality = info.LinkQuality
	if info.Stats != nil {
		m.TotalRxTx = sizeFormat(info.Stats.TransSend+info.Stats.P2PSend) + " / " + sizeFormat(info.Stats.TransReceive+info.Stats.P2PReceive)
		m.TransRxTx = sizeFormat(info.Stats.TransSend) + " / " + sizeFormat(info.Stats.TransReceive)
		m.P2PRxTx = sizeFormat(info.Stats.P2PSend) + " / " + sizeFormat(info.Stats.P2PReceive)
	}
	return m
}

func uint2IpV4(ip uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d", ip>>24, (ip>>16)&0xFF, (ip>>8)&0xFF, ip&0xFF)
}

func sizeFormat(size uint64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	} else if size < 1048576 {
		return fmt.Sprintf("%.2f KB", float32(size)/1024.0)
	} else if size < 1073741824 {
		return fmt.Sprintf("%.2f MB", float64(size)/1048576.0)
	} else {
		return fmt.Sprintf("%.2f GB", float64(size)/1073741824.0)
	}
}
