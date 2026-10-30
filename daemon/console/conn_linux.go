//go:build linux

package console

import (
	"errors"
	"net"
	"os"

	"github.com/mdlayher/socket"
)

type (
	vsockConn struct{ *socket.Conn }
	vsockAddr struct{}

	// timeoutError is a deadline passed, as a tcp connection reports it.
	timeoutError struct{ error }
)

func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

// connError reports a deadline passed as a tcp connection does, as a temporary
// net.Error, where the socket of a vsock connection is a file, and its error
// a path error. The tls layer keeps any other error for all the following
// reads, and the http server interrupts a read with a past deadline when a
// websocket takes the connection over: it would be the end of the session.
func connError(err error) error {
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return timeoutError{err}
	}
	return err
}

func (vsockAddr) Network() string { return "vsock" }
func (vsockAddr) String() string  { return "hypervisor" }

func (vsockConn) LocalAddr() net.Addr  { return vsockAddr{} }
func (vsockConn) RemoteAddr() net.Addr { return vsockAddr{} }

func (t vsockConn) Read(b []byte) (int, error) {
	n, err := t.Conn.Read(b)
	return n, connError(err)
}

func (t vsockConn) Write(b []byte) (int, error) {
	n, err := t.Conn.Write(b)
	return n, connError(err)
}

// inheritedConn returns the connection of the descriptor f. net.FileConn knows
// the tcp and unix sockets only: what it refuses here is the vsock connection
// of a hypervisor.
func inheritedConn(f *os.File) (net.Conn, error) {
	if c, err := net.FileConn(f); err == nil {
		return c, nil
	}
	c, err := socket.FileConn(f, "vsock")
	if err != nil {
		return nil, err
	}
	return vsockConn{c}, nil
}
