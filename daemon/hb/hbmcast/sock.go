package hbmcast

import (
	"net"

	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// dialMulticast returns a socket sending to the group gaddr on the
// interface ifi, or on the one the system routes the group to when ifi is
// nil.
//
// The interface is named by its index. The sender used to be bound to an
// address of the interface instead, so an interface with no address yet, as
// one the network manager had not brought up when the daemon started, left
// it unbound: the beats went out on the interface the group is routed to,
// while the receiver listened on the configured one, and so it stayed until
// the daemon restarted. By its index, the beats go out on the configured
// interface from the moment it carries the group. The receiver needs no
// such change: net.ListenMulticastUDP joins by index on Linux.
func dialMulticast(ifi *net.Interface, gaddr *net.UDPAddr) (*net.UDPConn, error) {
	c, err := net.ListenUDP(udpNetwork(gaddr), nil)
	if err != nil {
		return nil, err
	}
	if ifi != nil {
		if gaddr.IP.To4() != nil {
			err = ipv4.NewPacketConn(c).SetMulticastInterface(ifi)
		} else {
			err = ipv6.NewPacketConn(c).SetMulticastInterface(ifi)
		}
		if err != nil {
			_ = c.Close()
			return nil, err
		}
	}
	return c, nil
}

func udpNetwork(addr *net.UDPAddr) string {
	if addr.IP.To4() != nil {
		return "udp4"
	}
	return "udp6"
}

// hasAddrOf says the interface has an address of the family of the group:
// one with none is likely not configured yet, its network down.
func hasAddrOf(ifi *net.Interface, gaddr *net.UDPAddr) bool {
	addrs, err := ifi.Addrs()
	if err != nil {
		return false
	}
	want4 := gaddr.IP.To4() != nil
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		if (ipNet.IP.To4() != nil) == want4 {
			return true
		}
	}
	return false
}
