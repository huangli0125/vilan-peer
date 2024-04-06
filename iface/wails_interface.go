package iface

import (
	"vilan/model"
	"vilan/protocol"
)

type WailsInterface interface {
	UpdateState(state model.PeerState)
	UpdateStats(stats *protocol.Statistics)
	UpdatePeers()
}
