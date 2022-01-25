package network

import (
	"context"
	"fmt"
	"net"
	"strings"
)

func GetNodeAddr(nodename string, network string) (net.IP, error) {
	ips, err := net.DefaultResolver.LookupIP(context.Background(), network, nodename)
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		ipStr := ip.String()
		switch ipStr {
		case "127.0.0.1", "127.0.1.1", "::1":
			continue
		}
		if strings.HasPrefix(ipStr, "fe80:") {
			continue
		}
		return ip, nil
	}
	return nil, fmt.Errorf("node %s has no %s address", nodename, network)

}
