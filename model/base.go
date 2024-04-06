package model

const (
	SizeEthFrame  = 14
	SizeMaxPacket = 2048
	AESKey        = "vilan_hash_key"
)

type PeerState int32

const (
	StateUnInit    PeerState = 0
	StateInitError PeerState = 1
	StateInitOk    PeerState = 2
	StateUnConn    PeerState = 3
	StateConnOk    PeerState = 4 // 用于TCP
	StateUnAck     PeerState = 5
	StateAuthFail  PeerState = 6
	StateOk        PeerState = 7
)

type CryptType uint32

const (
	CryptNone CryptType = 0
	CryptAES  CryptType = 1
	CryptDES  CryptType = 2
	CryptRSA  CryptType = 3
)

type LinkMode uint32

const (
	LinkUnKnow LinkMode = 0
	LinkETH    LinkMode = 1
	LinkWiFi   LinkMode = 2
	LinkGprs   LinkMode = 3
)

//type NatType uint32
//
//const (
//	NatUnKnown   NatType = 0
//	NatFullCone  NatType = 1
//	NatAddrCone  NatType = 2
//	NatPortCone  NatType = 3
//	NatSymmetric NatType = 4
//)

const (
	RegResOk           = 0
	RegResParaErr      = -1
	RegResPwdErr       = -2
	RegResGroupNotAuto = -3
	RegResIpNotAuto    = -4
	RegResIpNotMatch   = -5
	RegResIpFull       = -6
	RegResDuplicate    = -7
	RegResDontAck      = -100
)
