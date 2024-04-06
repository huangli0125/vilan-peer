package iface

import (
	"vilan/model"
	"vilan/protocol"
)

type P2pInterface interface {
	Start() error
	Stop()
	TryForwardMessage(mac uint64, msg *protocol.MsgDataFrame) bool
	PeerStateChanged(mac uint64, online bool) error
	SendP2PTrigger(dstMac uint64) error
	ProcessP2PTrigger(msg *protocol.MsgP2PTrigger) error
	ProcessP2PAck(msgAck *protocol.MsgP2PAck) error
	IsP2P(dstMac uint64) bool
	P2PSuccess(dstMac uint64, sock *model.PeerSockContext) error
	DeleteFailedP2P(mac uint64)
	ResetP2PSocks()
}
