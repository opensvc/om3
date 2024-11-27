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

func ParseExpose(s string) (Expose, error) {
	var expose Expose
	l := strings.Split(s, "/")
	if n := len(l); n != 2 {
		return expose, fmt.Errorf("invalid syntax: expect one slash, got %d", n-1)
	}
	proto := l[1]
	if i := strings.Index(proto, ":"); i >= 0 {
		fePortStr := strings.Trim(proto[i:], ":")
		expose.Network = proto[:i]
		if fePort, err := strconv.Atoi(fePortStr); err == nil {
			expose.FrontendPort = fePort
		} else if fePort, err = net.LookupPort(proto, fePortStr); err != nil {
			return expose, err
		}
	} else {
		expose.Network = proto
	}
	switch expose.Network {
	case "tcp", "tcp4", "tcp6", "udp", "udp4", "udp6":
	default:
		return expose, fmt.Errorf("invalid syntax: expect network in tcp, tcp4, tcp6, udp, udp4, udp6. got %s", expose.Network)
	}
	if bePort, err := strconv.Atoi(l[0]); err != nil {
		return expose, err
	} else {
		expose.BackendPort = bePort
	}
	if expose.FrontendPort == 0 {
		expose.FrontendPort = expose.BackendPort
	}
	return expose, nil
}
