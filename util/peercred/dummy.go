//go:build !linux

package peercred

import "net"

// FromConn returns ErrNotUnix: the credentials of a unix socket peer are
// read on Linux only.
func FromConn(c net.Conn) (*T, error) {
	return nil, ErrNotUnix
}

// IDs returns ErrNotUnix: the credentials of a unix socket peer are read on
// Linux only.
func IDs(c net.Conn) (uint32, []uint32, error) {
	return 0, nil, ErrNotUnix
}
