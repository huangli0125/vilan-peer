package iface

import (
	"vilan/model"
	"vilan/protocol"
)

type RuntimeInterface interface {
	Init() error
	Start() error
	Stop()
	Restart() error
	PeerState() model.PeerState
	SetPeerState(state model.PeerState)
	PostTunTapData(dstMac uint64, data []byte) error
	EncryptMsg(msg *protocol.MsgDataFrame, out []byte) (int, error)
	DecryptMsg(msg *protocol.MsgDataFrame, out []byte) (int, error)
	SendAuthRequest() error
	ProcessAuthResponse(ack *protocol.MsgAuthAck) error
	SendUnAuth() error
	ProcessPeerStateChanged(state *protocol.MsgState) error
	UpdateConfig(peer *model.PeerInfo) map[string]interface{}
	ProcessConfig(conf *protocol.MsgConfig) error
	ProcessConfigAck(msgAck *protocol.MsgConfigAck) error
	SendPing() error
	ProcessPong(pong *protocol.MsgPong) error
	SendGroupPeersRequest(groupName string) error
	ProcessGroupPeersResponse(response *protocol.MsgGroupPeersResponse) error
	SendPeerLinksRequest(dstMac uint64) error
	ProcessPeerLinksRequest(request *protocol.MsgPeerLinksRequest) error
	ProcessPeerLinksResponse(response *protocol.MsgPeerLinksResponse) error
	GetGroupPeers(group string, request bool) []*protocol.PeerInfo
	FindPeer(mac uint64) *protocol.PeerInfo
	GetLinkInfos(mac uint64) []*protocol.LinkInfo
	AddRoute(mac uint64) bool
	CleanRoutes()
	SetStats(size uint64, rx bool, p2p bool)
	GetStats() *protocol.Statistics
}
