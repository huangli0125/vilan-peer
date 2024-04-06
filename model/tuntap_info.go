package model

type DevType uint32

const (
	TAP DevType = 0x01
	TUN DevType = 0x02
)

type IpMode uint32

const (
	AutoAssign IpMode = 0x01
	Static     IpMode = 0x02
)

type TunTapState uint32

const (
	TunTapUnInit     TunTapState = 0x01
	TunTapInitFailed TunTapState = 0x02
	TunTapInitOk     TunTapState = 0x03
	TunTapRunning    TunTapState = 0x04
	TunTapStop       TunTapState = 0x05
)
