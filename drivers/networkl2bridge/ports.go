package networkl2bridge

import (
	"fmt"
	"net"
	"slices"

	"github.com/vishvananda/netlink"

	"github.com/opensvc/om3/v3/util/plog"
)

// applyPorts enslaves to br the ports of s, or the vlan interfaces om
// creates on them, and releases the devices st records om enslaved that s no
// longer lists, deleting the vlan interfaces om created.
//
// A port is refused rather than enslaved when it holds an address, as the
// node's own interface on the segment, which enslaved would stop answering
// on it, cutting the node off; and when it is enslaved to another device. A
// port missing is an error: the network is set up again when its link comes
// up.
func applyPorts(log *plog.Logger, s settings, mtu int, br netlink.Link, st *state) error {
	want := make([]string, 0, len(s.ports))
	var errs []error
	for _, port := range s.ports {
		dev := port
		if s.vlan > 0 {
			dev = vlanName(port, s.vlan)
			if len(dev) >= maxDevNameLen {
				errs = append(errs, fmt.Errorf("the vlan interface name %s is longer than %d characters", dev, maxDevNameLen-1))
				continue
			}
			if err := ensureVLAN(log, port, dev, s.vlan, st); err != nil {
				errs = append(errs, err)
				continue
			}
		}
		want = append(want, dev)
		if err := enslavePort(log, dev, mtu, s.hairpin, br, st); err != nil {
			errs = append(errs, err)
		}
	}
	for _, dev := range slices.Clone(st.Ports) {
		if slices.Contains(want, dev) {
			continue
		}
		releasePort(log, dev, s.bridge)
		st.Ports = slices.DeleteFunc(st.Ports, func(s string) bool { return s == dev })
	}
	for _, dev := range slices.Clone(st.VLANs) {
		if slices.Contains(want, dev) {
			continue
		}
		if err := delLink(log, dev); err != nil {
			errs = append(errs, err)
			continue
		}
		st.VLANs = slices.DeleteFunc(st.VLANs, func(s string) bool { return s == dev })
	}
	if len(errs) > 0 {
		return fmt.Errorf("ports: %v", errs)
	}
	return nil
}

// ensureVLAN creates the vlan interface dev of id on port, recorded in st,
// unless it exists as such. A device of the name that is no vlan of id on
// port is an error.
func ensureVLAN(log *plog.Logger, port, dev string, id int, st *state) error {
	parent, err := netlink.LinkByName(port)
	if err != nil {
		return fmt.Errorf("port %s: %w", port, err)
	}
	if link, err := netlink.LinkByName(dev); err == nil {
		v, ok := link.(*netlink.Vlan)
		if !ok || v.VlanId != id || v.ParentIndex != parent.Attrs().Index {
			return fmt.Errorf("%s exists and is not the vlan %d of %s", dev, id, port)
		}
	} else if _, ok := err.(netlink.LinkNotFoundError); !ok {
		return err
	} else {
		la := netlink.NewLinkAttrs()
		la.Name = dev
		la.ParentIndex = parent.Attrs().Index
		if err := netlink.LinkAdd(&netlink.Vlan{LinkAttrs: la, VlanId: id}); err != nil {
			return fmt.Errorf("create the vlan %d of %s: %w", id, port, err)
		}
		log.Infof("created the vlan interface %s", dev)
		st.addVLAN(dev)
	}
	if slices.Contains(st.VLANs, dev) {
		if err := disableIPv6(dev); err != nil {
			return err
		}
	}
	if err := netlink.LinkSetUp(parent); err != nil {
		return fmt.Errorf("set %s up: %w", port, err)
	}
	return nil
}

// enslavePort enslaves dev to br, with the mtu and the hairpin mode of the
// network, recorded in st.
func enslavePort(log *plog.Logger, dev string, mtu int, hairpin bool, br netlink.Link, st *state) error {
	link, err := netlink.LinkByName(dev)
	if err != nil {
		return fmt.Errorf("port %s: %w", dev, err)
	}
	master := link.Attrs().MasterIndex
	if master != 0 && master != br.Attrs().Index {
		name := fmt.Sprint(master)
		if m, err := netlink.LinkByIndex(master); err == nil {
			name = m.Attrs().Name
		}
		return fmt.Errorf("port %s is enslaved to %s", dev, name)
	}
	if master == 0 {
		if addr, ok := hasAddress(link); ok {
			return fmt.Errorf("port %s holds the address %s, which the bridge would take from the node: bridge it in the network configuration of the node and name the bridge in dev", dev, addr)
		}
		if mtu > 0 && link.Attrs().MTU != mtu {
			if err := netlink.LinkSetMTU(link, mtu); err != nil {
				return fmt.Errorf("set the mtu of %s to %d: %w", dev, mtu, err)
			}
		}
		if err := netlink.LinkSetMaster(link, br); err != nil {
			return fmt.Errorf("enslave %s to %s: %w", dev, br.Attrs().Name, err)
		}
		log.Infof("enslaved %s to %s", dev, br.Attrs().Name)
	}
	st.addPort(dev)
	if err := netlink.LinkSetHairpin(link, hairpin); err != nil {
		return fmt.Errorf("set the hairpin mode of %s: %w", dev, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("set %s up: %w", dev, err)
	}
	return nil
}

// releasePort releases dev from the bridge named bridge, if still enslaved
// to it.
func releasePort(log *plog.Logger, dev, bridge string) {
	link, err := netlink.LinkByName(dev)
	if err != nil {
		return
	}
	br, err := netlink.LinkByName(bridge)
	if err != nil || link.Attrs().MasterIndex != br.Attrs().Index {
		return
	}
	if err := netlink.LinkSetNoMaster(link); err != nil {
		log.Warnf("release %s from %s: %s", dev, bridge, err)
		return
	}
	log.Infof("released %s from %s", dev, bridge)
}

// hasAddress returns an address of link that is not a link-local one: the
// node reaches the network through it.
func hasAddress(link netlink.Link) (net.IP, bool) {
	addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		return nil, false
	}
	for _, a := range addrs {
		if a.IP.IsLinkLocalUnicast() {
			continue
		}
		return a.IP, true
	}
	return nil, false
}
