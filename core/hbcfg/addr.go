package hbcfg

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

// ErrIntfRequired is the error of a multicast group that can only be joined
// on a named interface.
var ErrIntfRequired = errors.New("needs intf")

// CheckMulticastGroup says why a multicast heartbeat can not use the group
// addr on the interface intf, or nil.
//
// A group named by a name is checked on its zone alone here: its address is
// known once resolved, and is checked by CheckMulticastIP then.
func CheckMulticastGroup(addr, intf string) error {
	if strings.Contains(addr, "%") {
		return fmt.Errorf("the group %s names a zone: name the interface with intf", addr)
	}
	ip := net.ParseIP(addr)
	if ip == nil {
		return nil
	}
	return CheckMulticastIP(ip, intf)
}

// CheckMulticastIP says why a multicast heartbeat can not use the group ip on
// the interface intf, or nil.
//
// An IPv6 group needs an interface: every IPv6 multicast address is scoped,
// and the system has no route to pick an interface from for most of them, a
// link-local group as ff02::/16 first. Picking one for the operator would join
// the group where the peers may not be.
func CheckMulticastIP(ip net.IP, intf string) error {
	if !ip.IsMulticast() {
		return fmt.Errorf("%s is not a multicast group", ip)
	}
	if ip.To4() == nil && intf == "" {
		return fmt.Errorf("the IPv6 group %s %w", ip, ErrIntfRequired)
	}
	return nil
}

// CheckUnicastAddr says why a unicast heartbeat can not use the address
// addr, or nil.
//
// An IPv6 link-local address, with or without its zone, is not supported:
// it is only reachable through the interface its zone names, which the
// listener and the peers would each have to name for the others.
func CheckUnicastAddr(addr string) error {
	if strings.Contains(addr, "%") {
		return fmt.Errorf("the address %s names a zone: IPv6 link-local addresses are not supported, use a global or unique local address", addr)
	}
	ip := net.ParseIP(addr)
	if ip == nil {
		return nil
	}
	if ip.To4() == nil && ip.IsLinkLocalUnicast() {
		return fmt.Errorf("the address %s is link-local: IPv6 link-local addresses are not supported, use a global or unique local address", addr)
	}
	return nil
}
