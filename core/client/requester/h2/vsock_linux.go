//go:build linux

package reqh2

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/mdlayher/socket"
	"golang.org/x/sys/unix"
)

const (
	// vsockSourcePortMax and vsockSourcePortMin bound the reserved source
	// ports a connection binds, which the guest asks of its hypervisor.
	vsockSourcePortMax = 1023
	vsockSourcePortMin = 512
)

type (
	vsockConn struct{ *socket.Conn }
	vsockAddr string
)

func (t vsockAddr) Network() string { return "vsock" }
func (t vsockAddr) String() string  { return string(t) }

func (t vsockConn) LocalAddr() net.Addr  { return vsockAddr("local") }
func (t vsockConn) RemoteAddr() net.Addr { return vsockAddr("remote") }

// dialVsock connects to the port of the context id from a reserved source
// port. The bind needs CAP_NET_BIND_SERVICE, and a port in use the next one.
func dialVsock(ctx context.Context, cid, port uint32) (net.Conn, error) {
	c, err := socket.Socket(unix.AF_VSOCK, unix.SOCK_STREAM, 0, "vsock", nil)
	if err != nil {
		return nil, err
	}
	for p := uint32(vsockSourcePortMax); ; p-- {
		err = c.Bind(&unix.SockaddrVM{CID: unix.VMADDR_CID_ANY, Port: p})
		if err == nil {
			break
		}
		if !errors.Is(err, unix.EADDRINUSE) || p == vsockSourcePortMin {
			_ = c.Close()
			return nil, fmt.Errorf("bind a reserved vsock source port: %w", err)
		}
	}
	if _, err := c.Connect(ctx, &unix.SockaddrVM{CID: cid, Port: port}); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("connect to vsock %d:%d: %w", cid, port, err)
	}
	return vsockConn{c}, nil
}
