//go:build !linux

package netif

import (
	"fmt"
	"net"
)

func HasCarrier(_ string) (bool, error) {
	return false, fmt.Errorf("netif.HasCarrier() not implemented")
}

func AddAddr(_ string, _ *net.IPNet) error {
	return fmt.Errorf("netif.AddAddr() not implemented")
}

func DelAddr(_ string, _ *net.IPNet) error {
	return fmt.Errorf("netif.DelAddr() not implemented")
}

func InterfaceNameByIP(ref net.IP) (string, error) {
	return "", fmt.Errorf("netif.InterfaceNameByIP() not implemented")
}
