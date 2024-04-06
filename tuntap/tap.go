package tuntap

type TapDevice interface {
	Up() error
	Down() error
	Close() error
	Read([]byte) (int, error)
	Write([]byte) (int, error)

	SetMac(mac uint64) error
	GetMac() (uint64, error)
	SetIpAddr(addr, mask string) error
	GetIpAddr() (string, string, error)
	SetMtu(mtu uint) error
	Name() string
}
