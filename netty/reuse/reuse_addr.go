package reuse

import (
	"net"
)

// ResolveAddr parses given parameters to net.Addr.
func ResolveAddr(network, address string) (net.Addr, error) {
	switch network {
	case "ip", "ip4", "ip6":
		return net.ResolveIPAddr(network, address)
	case "tcp", "tcp4", "tcp6":
		return net.ResolveTCPAddr(network, address)
	case "udp", "udp4", "udp6":
		return net.ResolveUDPAddr(network, address)
	case "unix", "unixgram", "unixpacket":
		return net.ResolveUnixAddr(network, address)
	default:
		return nil, net.UnknownNetworkError(network)
	}
}
