package model

import (
	"time"
	"vilan/common"
	"vilan/netty"
	"vilan/protocol"
)

type ServerSockContext struct {
	Bootstrap   netty.Bootstrap
	Handler     netty.HandlerContext
	Token       uint32
	LastReceive int64
	Heartbeat   uint32
	Offline     uint32
	Connected   bool
}

func (s *ServerSockContext) IsConnected() bool {
	if !s.Connected {
		return false
	}
	now := time.Now().Unix()
	if now-s.LastReceive >= int64(s.Offline) {
		return false
	} else if now-s.LastReceive > int64(s.Heartbeat+3) {
		return isNetOk()
	} else {
		return true
	}
}

func isNetOk() bool {
	return common.Ping("www.baidu.com", 1000)
}

// p2p成功后的会话
type PeerSockContext struct {
	PeerMac         uint64 // 对端mac
	Bootstrap       netty.Bootstrap
	Handler         netty.HandlerContext
	LocalSock       *protocol.Sock
	TryConnCount    uint32 // 尝试发送次数
	LastTryConnTime int64
	LastReceive     int64
	PingTryCount    uint64 // 没有收到pong +1 大于3移除
	Heartbeat       uint32 // 心跳间隔时间
	Offline         uint32
	Connected       bool
}

func (s *PeerSockContext) IsConnected() bool {
	return s.Connected && uint32(time.Now().Unix()-s.LastReceive) < s.Offline
}

// 打洞使用的sock
type PunchSockContext struct {
	PeerMac     uint64 // 对端mac
	Bootstrap   netty.Bootstrap
	Handler     netty.HandlerContext
	LocalSock   *protocol.Sock
	StartTime   int64
	LastReceive int64
	Connected   bool
}
type PunchFailInfo struct {
	DstMac      uint64
	FailedCount uint32    // 失败次数 次数大于10次 不在允许继续
	FailedTime  time.Time // 失败时间
}

type MessageOut struct {
	Handler netty.HandlerContext
	//Dir        uint // 0 server 1 peer
	//Len        uint64
	MsgContent interface{}
}
