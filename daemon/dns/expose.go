package dns

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

type Expose struct {
	FrontendPort int
	BackendPort  int
	Network      string
}

// ParseExpose reads an expose declaration, written as a publish one is:
//
//	[<frontend port>:]<port>[/<proto>]
//
// The port is the one the address serves, the frontend port the one the
// SRV record is named after, the port itself when not set. The protocol is
// tcp when not set.
//
// The former form, <port>/<proto>[:<frontend port>], is still read: a colon
// after the slash says it.
func ParseExpose(s string) (Expose, error) {
	if i := strings.Index(s, "/"); i >= 0 && strings.Contains(s[i+1:], ":") {
		return parseLegacyExpose(s)
	}
	expose := Expose{Network: "tcp"}
	spec := s
	if i := strings.LastIndex(spec, "/"); i >= 0 {
		expose.Network = spec[i+1:]
		spec = spec[:i]
	}
	if err := checkExposeNetwork(expose.Network); err != nil {
		return expose, err
	}
	l := strings.Split(spec, ":")
	switch len(l) {
	case 1:
		l = []string{"", l[0]}
	case 2:
	default:
		return expose, fmt.Errorf("invalid syntax: expect [<frontend port>:]<port>[/<proto>], an expose names no address. got %s", s)
	}
	var err error
	if expose.BackendPort, err = strconv.Atoi(l[1]); err != nil {
		return expose, err
	}
	if expose.FrontendPort, err = parseFrontendPort(expose.Network, l[0]); err != nil {
		return expose, err
	}
	if expose.FrontendPort == 0 {
		expose.FrontendPort = expose.BackendPort
	}
	return expose, nil
}

// parseLegacyExpose reads the former form, <port>/<proto>[:<frontend port>].
func parseLegacyExpose(s string) (Expose, error) {
	var expose Expose
	l := strings.Split(s, "/")
	if n := len(l); n != 2 {
		return expose, fmt.Errorf("invalid syntax: expect one slash, got %d", n-1)
	}
	proto := l[1]
	fePortStr := ""
	if i := strings.Index(proto, ":"); i >= 0 {
		fePortStr = strings.Trim(proto[i:], ":")
		expose.Network = proto[:i]
	} else {
		expose.Network = proto
	}
	if err := checkExposeNetwork(expose.Network); err != nil {
		return expose, err
	}
	var err error
	if expose.FrontendPort, err = parseFrontendPort(expose.Network, fePortStr); err != nil {
		return expose, err
	}
	if expose.BackendPort, err = strconv.Atoi(l[0]); err != nil {
		return expose, err
	}
	if expose.FrontendPort == 0 {
		expose.FrontendPort = expose.BackendPort
	}
	return expose, nil
}

func checkExposeNetwork(network string) error {
	switch network {
	case "tcp", "tcp4", "tcp6", "udp", "udp4", "udp6":
		return nil
	default:
		return fmt.Errorf("invalid syntax: expect network in tcp, tcp4, tcp6, udp, udp4, udp6. got %s", network)
	}
}

// parseFrontendPort reads a frontend port, a number or a service name, and
// zero for none.
func parseFrontendPort(network, s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	if port, err := strconv.Atoi(s); err == nil {
		return port, nil
	}
	return net.LookupPort(network, s)
}
