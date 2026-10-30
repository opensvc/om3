//go:build !linux

package reqh2

import (
	"context"
	"errors"
	"net"
)

func DialVsock(_ context.Context, _, _ uint32) (net.Conn, error) {
	return nil, errors.New("vsock is not supported on this system")
}
