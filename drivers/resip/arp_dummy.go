//go:build solaris

package resip

import "net"

func gratuitousARP(ip net.IP, dev string) error {
	return ErrAnnounceUnsupported
}

func neighborAdvertisement(ip net.IP, dev string) error {
	return ErrAnnounceUnsupported
}
