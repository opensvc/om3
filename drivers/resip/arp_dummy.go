//go:build solaris

package resip

import "net"

func gratuitousARP(ip net.IP, dev string) error {
	return nil
}

func neighborAdvertisement(ip net.IP, dev string) error {
	return nil
}
