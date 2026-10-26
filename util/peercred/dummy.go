//go:build !linux

package peercred

import "net"

// FromConn returns ErrNotUnix: the credentials of a unix socket peer are
// read on Linux only.
func FromConn(c net.Conn) (*T, error) {
	return nil, ErrNotUnix
}
