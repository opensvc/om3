//go:build !linux

package console

import (
	"net"
	"os"
)

func inheritedConn(f *os.File) (net.Conn, error) {
	return net.FileConn(f)
}
