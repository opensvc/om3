//go:build linux

package resipnetns

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"

	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/resip"
	"github.com/opensvc/om3/v3/util/hostname"
)

// A layer 2 network, as a l2_bridge one, is a segment om neither addresses
// nor routes: a bridge with no address on each node. An ip.netns naming one
// joins the segment by a veth pair, one end in the namespace and the other a
// port of the bridge, in one of two ways:
//
//   - with no name, it stops there: the interface is up in the namespace
//     with no address, for the container to ask one of a dhcp server of the
//     segment, or to bridge or route the segment itself;
//   - with a name and a netmask, it adds that address to the interface.
//
// The node has no address on the segment, so nothing is set up for the node
// to reach the address, and nothing checked from the node.

// onL2 says the network keyword names a layer 2 network.
func (t *T) onL2() bool {
	nw, err := t.resolveNetwork()
	if err != nil || nw == nil {
		return false
	}
	return isLayer2(nw)
}

func isLayer2(nw network.Networker) bool {
	i, ok := nw.(network.Layer2er)
	return ok && i.IsLayer2()
}

// l2Only says the resource joins a layer 2 network and holds no address on
// it: its configuration names none, and the network has none to draw.
func (t *T) l2Only() bool {
	return t.Name == "" && t.onL2()
}

// setsNoDefaultRoute says the resource never sets the default route of the
// namespace: on a layer 2 network, a route through the interface alone would
// have the namespace look for every address on a segment whose router is not
// known, so only a gateway the configuration names makes one.
func (t *T) setsNoDefaultRoute() bool {
	return t.Gateway == "" && t.onL2()
}

// configureL2 takes from a layer 2 network the bridge the namespace is
// plugged into, and refuses what the configuration asks that such a network
// can not give.
//
// The refusals are errors of the configuration, as the one of a network
// providing neither a device nor an address is: the validation of the
// resources reports them, and no action runs on the resource. They are its
// ConfigError too while it holds nothing in the namespace, so the change
// fixing them is not refused for a status not known.
func (t *T) configureL2(nw network.Networker) error {
	bridge := t.networkDev()
	switch {
	case bridge == "":
		return fmt.Errorf("network %s names no bridge to plug the namespace into", nw.Name())
	case t.Mode != "bridge" || t.Tags.Has(tagDedicated):
		mode := t.Mode
		if t.Tags.Has(tagDedicated) {
			mode = "dedicated"
		}
		return fmt.Errorf("mode %s is not supported on network %s, a layer 2 segment joined through its bridge %s: unset mode, or set it to bridge", mode, nw.Name(), bridge)
	case t.Dev != "" && t.Dev != bridge:
		return fmt.Errorf("dev %s is not the bridge of network %s, %s: unset dev, the network says it", t.Dev, nw.Name(), bridge)
	}
	t.Dev = bridge
	if t.MacAddr != "" {
		if _, err := net.ParseMAC(t.MacAddr); err != nil {
			return fmt.Errorf("macaddr: %w", err)
		}
	}
	if len(t.Publish) > 0 {
		return fmt.Errorf("publish is not supported on network %s, a layer 2 segment the node has no address on: the node can not forward a port to an address it does not reach", nw.Name())
	}
	if t.Name != "" {
		if t.Netmask == "" {
			return fmt.Errorf("netmask is not set, and network %s, a layer 2 segment om does not address, has none to give: set the netmask of %s", nw.Name(), t.Name)
		}
		if t.Gateway != "" {
			t.gatewayRank = gatewayOwn
		}
		return nil
	}
	// No name: the interface holds no address, and what needs one has
	// nothing to work with.
	needy := make([]string, 0)
	if exposes(t.Expose) {
		needy = append(needy, "expose")
	}
	if t.Gateway != "" {
		needy = append(needy, "gateway")
	}
	if len(needy) > 0 {
		return fmt.Errorf("%s needs an address, and the resource has none on network %s, a layer 2 segment om does not address: set the name and the netmask of a static address, or unset %s", strings.Join(needy, " and "), nw.Name(), strings.Join(needy, " and "))
	}
	return nil
}

// exposes says the expose keyword asks for a record: none asks for nothing.
func exposes(l []string) bool {
	return len(l) > 0 && !(len(l) == 1 && l[0] == "none")
}

// stableMAC returns a locally administered unicast hardware address derived
// from the id of the object and the id of the resource, the same at every
// start and on every node the object runs on: a dhcp server of the segment
// reserves an address for it, and the neighbors of a static address keep the
// entry they learned. The kernel gives a veth a random address otherwise, a
// new one at each start.
//
// The instances of a flex object run at once, on a segment the nodes may
// share: nodename, set for them, gives each its own address.
func stableMAC(objectID, rid, nodename string) net.HardwareAddr {
	sum := sha256.Sum256([]byte(objectID + "\x00" + rid + "\x00" + nodename))
	mac := make(net.HardwareAddr, 6)
	copy(mac, sum[:6])
	// Locally administered, unicast.
	mac[0] = mac[0]&0xfc | 0x02
	return mac
}

// macAddr returns the hardware address to set on the interface of the
// namespace: the one the macaddr keyword names, else a stable one on a layer
// 2 network, else none, the one the kernel chose being left.
func (t *T) macAddr() string {
	if t.MacAddr != "" || !t.onL2() {
		return t.MacAddr
	}
	var nodename string
	if resip.PerInstance(t.Shared, t.Topology) {
		nodename = hostname.Hostname()
	}
	return stableMAC(t.ObjectID.String(), t.RID(), nodename).String()
}

// disableIPv6 disables ipv6 on the device dev of the node. A kernel with no
// ipv6 has nothing to disable. A variable for the tests.
var disableIPv6 = func(dev string) error {
	p := filepath.Join("/proc/sys/net/ipv6/conf", dev, "disable_ipv6")
	if err := os.WriteFile(p, []byte("1"), 0o644); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("disable ipv6 on %s: %w", dev, err)
	}
	return nil
}

// checkL2Bridge says the bridge of the layer 2 network is missing on this
// node, in words naming the cause: the network is not set up here.
func (t *T) checkL2Bridge() error {
	if _, err := t.netInterface(); err != nil {
		return fmt.Errorf("the bridge %s of network %s does not exist on this node: set the network up, with om network setup", t.Dev, t.Network)
	}
	if !isLinuxBridge(t.Dev) {
		return fmt.Errorf("%s, the device of network %s, is not a bridge on this node", t.Dev, t.Network)
	}
	return nil
}

// l2Link is the interface of the resource in the namespace, as l2GuestLink
// finds it.
type l2Link struct {
	// name is the name of the interface in the namespace.
	name string

	// peer is the name of the other end of the pair on the node, empty
	// when not found there, and master the device it is a port of, empty
	// when it is a port of none.
	peer   string
	master string
}

// l2GuestLink finds the interface of the resource in the namespace by its
// hardware address, the only thing of the resource it holds, and the other
// end of its pair on the node. It returns nil when the namespace has no such
// interface.
//
// It is called from the namespace of the node, which the other end is looked
// up in by its index.
func l2GuestLink(netns ns.NetNS, mac string) (*l2Link, error) {
	hw, err := net.ParseMAC(mac)
	if err != nil {
		return nil, err
	}
	var (
		found     *l2Link
		index     int
		peerIndex int
	)
	if err := netns.Do(func(_ ns.NetNS) error {
		links, err := netlink.LinkList()
		if err != nil {
			return err
		}
		for _, link := range links {
			attrs := link.Attrs()
			if link.Type() != "veth" || attrs.HardwareAddr.String() != hw.String() {
				continue
			}
			found = &l2Link{name: attrs.Name}
			index, peerIndex = attrs.Index, attrs.ParentIndex
			return nil
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if found == nil || peerIndex == 0 {
		return found, nil
	}
	peer, err := netlink.LinkByIndex(peerIndex)
	if err != nil || peer.Type() != "veth" || peer.Attrs().ParentIndex != index {
		// The other end is not in the namespace of the node: the index
		// is the one of a device of another namespace.
		return found, nil
	}
	found.peer = peer.Attrs().Name
	if idx := peer.Attrs().MasterIndex; idx != 0 {
		if master, err := netlink.LinkByIndex(idx); err == nil {
			found.master = master.Attrs().Name
		}
	}
	return found, nil
}

// startL2Only joins the layer 2 network and stops there: a veth pair, one
// end up in the namespace with the hardware address of the resource, the
// other a port of the bridge of the network. No address, no route, no
// announce: the namespace has nothing to announce, and the node nothing to
// reach.
func (t *T) startL2Only(ctx context.Context) error {
	if err := t.checkL2Bridge(); err != nil {
		return err
	}
	pid, err := t.getNSPID(ctx)
	if err != nil {
		return err
	}
	netns, err := t.getNS(ctx)
	if err != nil {
		return err
	}
	if netns == nil {
		return fmt.Errorf("the network namespace of %s is not found: is it started ?", t.NetNS)
	}
	defer func() { _ = netns.Close() }()
	mac := t.macAddr()
	link, err := l2GuestLink(netns, mac)
	if err != nil {
		return err
	}
	if link != nil {
		switch {
		case link.master == t.Dev:
			t.Log().Infof("%s is already in the namespace, its peer %s a port of %s", link.name, link.peer, t.Dev)
			return nil
		case link.peer == "":
			return fmt.Errorf("%s is in the namespace with the hardware address %s, and its peer is not found on this node: stop the resource to delete it", link.name, mac)
		}
		// A port taken off the bridge, as by a network torn down and
		// set up again: plugged back.
		return t.linkSetMaster(link.peer, t.Dev)
	}
	guestDev, err := t.newGuestDev(netns)
	if err != nil {
		return err
	}
	if t.hasNSDev(netns) {
		return fmt.Errorf("%s is in the namespace already, with another hardware address than %s: set nsdev to a name of its own", guestDev, mac)
	}
	hostDev := formatHostDevName(guestDev, pid)
	if err := t.makeL2Pair(ctx, netns, hostDev, guestDev, pid); err != nil {
		return err
	}
	return nil
}

// makeL2Pair makes the veth pair of a layer 2 network: guestDev up in the
// namespace of the process pid, hostDev a port of the bridge of the network.
//
// The end on the node has ipv6 disabled before it is up: the node is not to
// hold an address on the segment, a link local one included, nor to take a
// route a router of the segment, or a container claiming to be one,
// advertises.
func (t *T) makeL2Pair(ctx context.Context, netns ns.NetNS, hostDev, guestDev string, pid int) error {
	mtu, err := t.devMTU()
	if err != nil {
		return err
	}
	tmpGuestDev := fmt.Sprintf("v%spg%d", guestDev, pid)
	if err := t.makeVethPair(hostDev, tmpGuestDev, mtu); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.linkDel(hostDev)
	})
	if err := disableIPv6(hostDev); err != nil {
		return err
	}
	if err := t.linkSetNsPidAndNameAndUp(tmpGuestDev, pid, guestDev); err != nil {
		_ = t.linkDel(tmpGuestDev)
		return err
	}
	if err := t.linkSetMacIn(guestDev, t.macAddr(), netns.Path()); err != nil {
		return err
	}
	return t.linkSetMaster(hostDev, t.Dev)
}

// stopL2Only deletes the veth pair of the resource, found by the hardware
// address of its end in the namespace. A namespace gone took the pair with
// it.
func (t *T) stopL2Only(ctx context.Context) error {
	netns, err := t.getNS(ctx)
	if err != nil {
		return err
	}
	if netns == nil {
		return nil
	}
	defer func() { _ = netns.Close() }()
	link, err := l2GuestLink(netns, t.macAddr())
	if err != nil {
		return err
	}
	if link == nil {
		t.Log().Infof("already down: no interface of the hardware address %s in the namespace", t.macAddr())
		return nil
	}
	// Deleting an end deletes the pair.
	return t.linkDelIn(link.name, netns.Path())
}

// statusL2Only is up when the interface of the resource is in the namespace
// and its peer is a port of the bridge of the network.
func (t *T) statusL2Only(ctx context.Context) status.T {
	if err := t.checkL2Bridge(); err != nil {
		t.StatusLog().Error("%s", err)
		return status.Down
	}
	netns, err := t.getNS(ctx)
	if err != nil {
		t.StatusLog().Error("netns: %s", err)
		return status.Down
	}
	if netns == nil {
		return status.Down
	}
	defer func() { _ = netns.Close() }()
	link, err := l2GuestLink(netns, t.macAddr())
	if err != nil {
		t.StatusLog().Error("guest dev: %s", err)
		return status.Down
	}
	switch {
	case link == nil:
		return status.Down
	case link.peer == "":
		t.StatusLog().Error("%s is in the namespace, and its peer is not found on this node", link.name)
		return status.Down
	case link.master != t.Dev:
		t.StatusLog().Error("%s is in the namespace, and its peer %s is not a port of %s, the bridge of network %s", link.name, link.peer, t.Dev, t.Network)
		return status.Down
	}
	t.StatusLog().Info("%s %s, no address: layer 2 only", link.name, t.macAddr())
	return status.Up
}

// arpProbeTimeout is how long the probes wait for the answer of another
// holder of the address.
const arpProbeTimeout = time.Second

// arpProbe asks the segment of dev who holds ip, from the namespace of the
// caller, and returns the hardware address answering, nil when none does. A
// variable for the tests.
var arpProbe = func(ip net.IP, dev string) (net.HardwareAddr, error) {
	return sendARPProbes(ip, dev, arpProbeTimeout)
}

// checkL2Duplicate refuses an ipv4 address another host of the layer 2
// segment answers for, asked by arp probes from the namespace before the
// address is added to the interface: the address announced would take the
// traffic of that host.
//
// It stands for the ping the start of an address is aborted by on the other
// networks, sent from the node: the node has no address on a layer 2
// segment, and its ping would ask another network, the one its routes lead
// to. An ipv6 address is checked by the kernel, its duplicate address
// detection.
//
// A resource tagged nonrouted is not checked, as its start is not aborted by
// a ping. A probe that can not be sent is warned of, and refuses nothing.
func (t *T) checkL2Duplicate(netns ns.NetNS, guestDev string) error {
	ip := t.ipaddr()
	if ip.To4() == nil || t.Tags.Has(tagNonRouted) {
		return nil
	}
	t.Log().Infof("ask the segment of network %s who holds %s, by arp over %s", t.Network, ip, guestDev)
	var holder net.HardwareAddr
	if err := netns.Do(func(_ ns.NetNS) error {
		var err error
		holder, err = arpProbe(ip, guestDev)
		return err
	}); err != nil {
		t.Log().Warnf("arp probe of %s on %s: %s", ip, guestDev, err)
		return nil
	}
	if holder == nil || strings.EqualFold(holder.String(), t.macAddr()) {
		return nil
	}
	return fmt.Errorf("%s is already held on the segment of network %s, by %s", ip, t.Network, holder)
}
