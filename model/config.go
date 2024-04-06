package model

import "vilan/common"

type Config struct {
	ServerIp       string    `json:"server_ip"`
	ServerPort     int32     `json:"server_port"`
	PeerName       string    `json:"peer_name"`
	GroupName      string    `json:"group_name"`
	GroupPwd       string    `json:"group_pwd"`
	CryptType      CryptType `json:"crypt_type"`
	PeerPwd        string    `json:"peer_pwd"`
	TabName        string    `json:"tab_name"`
	HwMac          string    `json:"hw_mac"`
	IpMode         uint      `json:"ip_mode"`
	IpAddr         string    `json:"ip_addr"`
	IpMask         string    `json:"ip_mask"`
	AllowVisitPort bool      `json:"allow_visit_port"`
	EnableLog      bool      `json:"enable_log"`
	LogLevel       byte      `json:"log_level"`
}

type PeerConfig struct {
	GroupName string    `json:"group_name"`
	GroupPwd  string    `json:"group_pwd"`
	PeerPwd   string    `json:"peer_pwd"`
	Name      string    `json:"name"`
	CryptType CryptType `json:"crypt_type"`
}
type TapConfig struct {
	Name      string  `json:"name"`
	HwMac     uint64  `json:"-"`
	HwMacStr  string  `json:"hw_mac"`
	IpMode    IpMode  `json:"ip_mode"`
	IpAddr    uint32  `json:"-"`
	IpAddrStr string  `json:"ip_addr"`
	IpMask    uint32  `json:"ip_mask_len"`
	DevType   DevType `json:"dev_type"`
}

type AppConfig struct {
	ServerIp         string         `json:"server_ip"`
	ServerPort       int32          `json:"server_port"`
	Heartbeat        uint32         `json:"heartbeat"`
	Offline          uint32         `json:"offline"`
	MaxPacketSize    int32          `json:"max_packet_size"`
	PacketNum        int            `json:"packet_num"`
	P2pTryCount      uint32         `json:"p2p_try_count"`
	P2pRetryInterval uint32         `json:"p2p_retry_interval"`
	AllowVisitPort   bool           `json:"allow_visit_port"`
	EnableLog        bool           `json:"enable_log"`
	SaveLog          bool           `json:"save_log"`
	LogLevel         common.LogType `json:"log_level"`
	PeerConfig       *PeerConfig    `json:"peer_config"`
	TapConfig        *TapConfig     `json:"tap_config"`
}
