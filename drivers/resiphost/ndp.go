//go:build !solaris

package resiphost

import (
	"fmt"
	"net"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv6"
)

// neighborAdvertise sends an unsolicited neighbor advertisement of the address
// to all the nodes of the link, overriding their cache, as an ipv4 gratuitous
// arp does: the neighbors of a moved address send to its new node at once,
// rather than to the node it left until their cache entry fails.
func (t *T) neighborAdvertise(dev string) error {
	ip := t.ipaddr().To16()
	ifi, err := net.InterfaceByName(dev)
	if err != nil {
		return err
	}
	c, err := icmp.ListenPacket("ip6:ipv6-icmp", "::")
	if err != nil {
		return fmt.Errorf("neighbor advertisement: %w", err)
	}
	defer func() { _ = c.Close() }()
	p := c.IPv6PacketConn()

	// The override flag, the target address, and the link-layer address of
	// the interface the target is on, which the neighbors learn.
	body := make([]byte, 20, 28)
	body[0] = 0x20
	copy(body[4:20], ip)
	if mac := ifi.HardwareAddr; len(mac) == 6 {
		body = append(body, 2, 1)
		body = append(body, mac...)
	}
	b, err := (&icmp.Message{
		Type: ipv6.ICMPTypeNeighborAdvertisement,
		Body: &icmp.RawBody{Data: body},
	}).Marshal(nil)
	if err != nil {
		return err
	}
	// A neighbor discovery message is valid only with a hop limit of 255,
	// which proves it was not routed. The kernel computes the checksum.
	cm := &ipv6.ControlMessage{HopLimit: 255, Src: ip, IfIndex: ifi.Index}
	dst := &net.IPAddr{IP: net.IPv6linklocalallnodes, Zone: dev}
	if _, err := p.WriteTo(b, cm, dst); err != nil {
		return fmt.Errorf("neighbor advertisement: %w", err)
	}
	return nil
}
