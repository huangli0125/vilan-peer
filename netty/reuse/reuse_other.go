//go:build !windows && !linux && !darwin && !dragonfly && !freebsd && !netbsd && !openbsd
// +build !windows,!linux,!darwin,!dragonfly,!freebsd,!netbsd,!openbsd

package reuse

import (
	"syscall"
)

// See net.RawConn.Control
func Control(network, address string, c syscall.RawConn) (err error) {
	return nil
}
