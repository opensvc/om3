package networkl2bridge

import (
	"errors"
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/opensvc/om3/v3/util/plog"
)

// zeroMAC is the address of the forwarding entries of a vxlan sending the
// broadcast, unknown unicast and multicast frames to a peer: one per peer,
// head-end replication, which needs no multicast of the underlay.
var zeroMAC = net.HardwareAddr{0, 0, 0, 0, 0, 0}

// vxlanMTU returns the mtu of the bridge of a vxlan sending from local: the
// one of the interface of the node holding local, less what vxlan adds.
func vxlanMTU(local net.IP) (int, error) {
	family := netlink.FAMILY_V4
	overhead := vxlanOverhead4
	if local.To4() == nil {
		family = netlink.FAMILY_V6
		overhead = vxlanOverhead6
	}
	addrs, err := netlink.AddrList(nil, family)
	if err != nil {
		return 0, err
	}
	for _, a := range addrs {
		if !a.IP.Equal(local) {
			continue
		}
		link, err := netlink.LinkByIndex(a.LinkIndex)
		if err != nil {
			return 0, err
		}
		return link.Attrs().MTU - overhead, nil
	}
	return 0, fmt.Errorf("the vxlan address %s is not an address of this node: set vxlan_addr", local)
}

// sameVXLAN says whether link is a vxlan of the identifier, port and local
// address of s: those can not be changed on an existing device.
func sameVXLAN(link netlink.Link, s settings) bool {
	v, ok := link.(*netlink.Vxlan)
	return ok && v.VxlanId == s.vni && v.Port == s.vxlanPort && v.SrcAddr.Equal(s.local)
}

// applyVXLAN enslaves to br the vxlan device of s, created or created again
// when its identifier, port or local address changed, with a forwarding
// entry to each peer and none to a node no longer one. Without a vxlan in s,
// the device st records om created is deleted.
func applyVXLAN(log *plog.Logger, s settings, mtu int, br netlink.Link, st *state) error {
	name := vxlanName(s.name)
	if s.vni == 0 {
		if st.VXLAN != "" {
			if err := delLink(log, st.VXLAN); err != nil {
				return err
			}
			st.VXLAN = ""
		}
		return nil
	}
	if st.VXLAN != "" && st.VXLAN != name {
		if err := delLink(log, st.VXLAN); err != nil {
			return err
		}
	}
	link, err := netlink.LinkByName(name)
	switch {
	case err == nil && sameVXLAN(link, s):
	case err == nil:
		if err := netlink.LinkDel(link); err != nil {
			return fmt.Errorf("delete %s to create it again: %w", name, err)
		}
		log.Infof("deleted %s, its identifier, port or local address changed", name)
		link = nil
	default:
		if _, ok := err.(netlink.LinkNotFoundError); !ok {
			return err
		}
		link = nil
	}
	if link == nil {
		la := netlink.NewLinkAttrs()
		la.Name = name
		if mtu > 0 {
			la.MTU = mtu
		}
		v := &netlink.Vxlan{
			LinkAttrs: la,
			VxlanId:   s.vni,
			SrcAddr:   s.local,
			Port:      s.vxlanPort,
			Learning:  true,
		}
		if err := netlink.LinkAdd(v); err != nil {
			return fmt.Errorf("create the vxlan %s: %w", name, err)
		}
		log.Infof("created the vxlan %s, vni %d, from %s port %d", name, s.vni, s.local, s.vxlanPort)
		if link, err = netlink.LinkByName(name); err != nil {
			return err
		}
	}
	st.VXLAN = name
	if mtu > 0 && link.Attrs().MTU != mtu {
		if err := netlink.LinkSetMTU(link, mtu); err != nil {
			return fmt.Errorf("set the mtu of %s to %d: %w", name, mtu, err)
		}
	}
	if link.Attrs().MasterIndex != br.Attrs().Index {
		if err := netlink.LinkSetMaster(link, br); err != nil {
			return fmt.Errorf("enslave %s to %s: %w", name, s.bridge, err)
		}
		log.Infof("enslaved %s to %s", name, s.bridge)
	}
	if err := netlink.LinkSetHairpin(link, s.hairpin); err != nil {
		return fmt.Errorf("set the hairpin mode of %s: %w", name, err)
	}
	if err := disableIPv6(name); err != nil {
		return err
	}
	if err := netlink.LinkSetUp(link); errors.Is(err, unix.EADDRINUSE) {
		return fmt.Errorf("set %s up: the udp port %d is bound by another vxlan, as the one of openvswitch: set vxlan_port to another port", name, s.vxlanPort)
	} else if err != nil {
		return fmt.Errorf("set %s up: %w", name, err)
	}
	return syncFDB(log, link, s.peers)
}

// fdbDsts returns the peers link has a forwarding entry of the zero address
// to.
func fdbDsts(link netlink.Link) ([]net.IP, error) {
	neighs, err := netlink.NeighList(link.Attrs().Index, unix.AF_BRIDGE)
	if err != nil {
		return nil, err
	}
	var l []net.IP
	for _, n := range neighs {
		if n.IP == nil || n.HardwareAddr.String() != zeroMAC.String() {
			continue
		}
		l = append(l, n.IP)
	}
	return l, nil
}

// fdbDiff returns the peers of want with no entry in have, and the entries
// of have to no peer of want.
func fdbDiff(have, want []net.IP) (add, del []net.IP) {
	contains := func(l []net.IP, ip net.IP) bool {
		for _, e := range l {
			if e.Equal(ip) {
				return true
			}
		}
		return false
	}
	for _, ip := range want {
		if !contains(have, ip) {
			add = append(add, ip)
		}
	}
	for _, ip := range have {
		if !contains(want, ip) {
			del = append(del, ip)
		}
	}
	return add, del
}

// fdbNeigh returns the forwarding entry of link sending to the peer ip.
func fdbNeigh(link netlink.Link, ip net.IP) *netlink.Neigh {
	return &netlink.Neigh{
		LinkIndex:    link.Attrs().Index,
		Family:       unix.AF_BRIDGE,
		Flags:        netlink.NTF_SELF,
		State:        netlink.NUD_PERMANENT | netlink.NUD_NOARP,
		HardwareAddr: zeroMAC,
		IP:           ip,
	}
}

// syncFDB makes the forwarding entries of link the ones to peers.
func syncFDB(log *plog.Logger, link netlink.Link, peers []net.IP) error {
	have, err := fdbDsts(link)
	if err != nil {
		return fmt.Errorf("list the forwarding entries of %s: %w", link.Attrs().Name, err)
	}
	add, del := fdbDiff(have, peers)
	for _, ip := range add {
		if err := netlink.NeighAppend(fdbNeigh(link, ip)); err != nil {
			return fmt.Errorf("add the forwarding entry of %s to %s: %w", link.Attrs().Name, ip, err)
		}
		log.Infof("added the forwarding entry of %s to %s", link.Attrs().Name, ip)
	}
	for _, ip := range del {
		if err := netlink.NeighDel(fdbNeigh(link, ip)); err != nil {
			return fmt.Errorf("delete the forwarding entry of %s to %s: %w", link.Attrs().Name, ip, err)
		}
		log.Infof("deleted the forwarding entry of %s to %s", link.Attrs().Name, ip)
	}
	return nil
}
