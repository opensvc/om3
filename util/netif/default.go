// +build !linux

package netif

import (
	"errors"
	"net"
)

func HasCarrier(_ string) (bool, error) {
	return false, errors.New("netif.HasCarrier() not implemented")
}

func AddAddr(_ string, _ *net.IPNet) error {
	return errors.New("netif.AddAddr() not implemented")
}

func DelAddr(_ string, _ *net.IPNet) error {
	return errors.New("netif.DelAddr() not implemented")
}

func InterfaceNameByIP(ref net.IP) (string, error) {
	return "", errors.New("netif.InterfaceNameByIP() not implemented")
}
