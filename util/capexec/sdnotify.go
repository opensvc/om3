package capexec

import (
	"fmt"
	"net"
	"os"
)

// sdNotifyReady tells the systemd service unit whose main process this is
// that it started, through the socket systemd names in NOTIFY_SOCKET. A
// process started otherwise has no such socket, which is an error: the
// unit would wait for the message until its start timeout.
func sdNotifyReady() error {
	socket := os.Getenv("NOTIFY_SOCKET")
	if socket == "" {
		return fmt.Errorf("--sd-notify: no NOTIFY_SOCKET: om exec is not the main process of a systemd service of type notify")
	}
	conn, err := net.DialUnix("unixgram", nil, &net.UnixAddr{Name: socket, Net: "unixgram"})
	if err != nil {
		return fmt.Errorf("--sd-notify: %w", err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.Write([]byte("READY=1")); err != nil {
		return fmt.Errorf("--sd-notify: %w", err)
	}
	return nil
}
