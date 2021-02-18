package listener

import (
	"net"
)

// Render formats a listener string, taking care of bracketing IPV6 addresses.
// Examples: [::]:1215, :1215 or 127.0.0.1:1215
func Render(addr net.IP, port int) string {
	a := net.TCPAddr{IP: addr, Port: port}
	return a.String()
}
