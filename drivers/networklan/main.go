// Package networklan is the lan network driver: a segment the nodes share,
// of which om hands out the addresses of the ranges it is given to the
// cluster rather than to a node, for an object to take its address along to
// the node it moves to, as the floating address of a failover service.
//
// Nothing is set up on the nodes: the addresses are configured on an
// interface of the node on the segment, which is reached as the rest of the
// segment is, and leave it as they are, the hosts of the segment answering
// them directly.
package networklan

import (
	"fmt"
	"net"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/ipam"
	"github.com/opensvc/om3/v3/core/network"
)

type (
	T struct {
		network.T
	}
)

var (
	drvID = driver.NewID(driver.GroupNetwork, "lan")

	// interfaces returns the interfaces of the node, replaced by the tests.
	interfaces = net.Interfaces

	// addrsOf returns the addresses of an interface, replaced by the tests.
	addrsOf = func(i net.Interface) ([]net.Addr, error) { return i.Addrs() }
)

func init() {
	driver.Register(drvID, NewNetworker)
}

func NewNetworker() network.Networker {
	t := New()
	var i interface{} = t
	return i.(network.Networker)
}

func New() *T {
	return &T{}
}

// AllocatableRange returns the segment, whatever the node: every node draws
// from the same ranges of it, so an object draws the same address wherever it
// runs.
func (t *T) AllocatableRange(_ string) (*net.IPNet, error) {
	return t.IPNet()
}

// Pools returns the ranges of the segment om hands out, which the ranges
// keyword lists. It is required: the segment holds the addresses of the
// nodes, of the router and of the other hosts, which om knows nothing of.
func (t *T) Pools() ([]ipam.Pool, error) {
	segment, err := t.IPNet()
	if err != nil {
		return nil, err
	}
	l := t.GetStrings("ranges")
	if len(l) == 0 {
		return nil, fmt.Errorf("network#%s.ranges is not set: list the addresses of the segment %s om hands out, as 192.168.10.100-192.168.10.199 or 192.168.10.128/26", t.Name(), segment)
	}
	pools, err := ipam.ParsePools(l)
	if err != nil {
		return nil, fmt.Errorf("network#%s.ranges: %w", t.Name(), err)
	}
	for _, p := range pools {
		if !p.In(segment) {
			return nil, fmt.Errorf("network#%s.ranges: %s is not in the segment %s", t.Name(), p, segment)
		}
	}
	return pools, nil
}

// IsClusterWide says the addresses of a lan network belong to the cluster.
func (t *T) IsClusterWide() bool {
	return true
}

// Netmask returns the prefix length of the segment, which the addresses drawn
// from it are configured with.
func (t *T) Netmask() (int, error) {
	segment, err := t.IPNet()
	if err != nil {
		return 0, err
	}
	ones, _ := segment.Mask.Size()
	return ones, nil
}

// Gateway returns the router of the segment, the gateway keyword, nil when
// it is not set. It must be an address of the segment, and not one om hands
// out.
func (t *T) Gateway() (net.IP, error) {
	s := t.GetString("gateway")
	if s == "" {
		return nil, nil
	}
	ip := net.ParseIP(s)
	if ip == nil {
		return nil, fmt.Errorf("network#%s.gateway %q is not an ip address", t.Name(), s)
	}
	segment, err := t.IPNet()
	if err != nil {
		return nil, err
	}
	if !segment.Contains(ip) {
		return nil, fmt.Errorf("network#%s.gateway %s is not on the segment %s", t.Name(), ip, segment)
	}
	pools, err := t.Pools()
	if err != nil {
		return nil, err
	}
	for _, p := range pools {
		if p.Contains(ip) {
			return nil, fmt.Errorf("network#%s.gateway %s is in the range %s, which om hands out", t.Name(), ip, p)
		}
	}
	return ip, nil
}

// HostDev returns the interface of this node on the segment: the dev keyword,
// or else the interface holding an address of the segment.
func (t *T) HostDev() (string, error) {
	if s := t.GetString("dev"); s != "" {
		return s, nil
	}
	segment, err := t.IPNet()
	if err != nil {
		return "", err
	}
	l, err := interfaces()
	if err != nil {
		return "", err
	}
	devs := make([]string, 0)
	for _, i := range l {
		if i.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := addrsOf(i)
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip, _, err := net.ParseCIDR(addr.String())
			if err != nil || !segment.Contains(ip) || t.holds(ip) {
				continue
			}
			devs = append(devs, i.Name)
			break
		}
	}
	switch len(devs) {
	case 0:
		return "", fmt.Errorf("no interface of this node holds an address of the segment %s of the network %s: set the dev keyword of the network", segment, t.Name())
	case 1:
		return devs[0], nil
	default:
		return "", fmt.Errorf("the interfaces %v of this node hold an address of the segment %s of the network %s: set the dev keyword of the network", devs, segment, t.Name())
	}
}

// holds says ip is one om hands out, which an interface holding says nothing
// of it being the interface of the node on the segment: a service address
// left on another interface would make it look like one.
func (t *T) holds(ip net.IP) bool {
	pools, err := t.Pools()
	if err != nil {
		return false
	}
	for _, p := range pools {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// Setup checks the node is on the segment, and the ranges are, which is all
// a lan network needs of it.
func (t *T) Setup() error {
	if _, err := t.Pools(); err != nil {
		return err
	}
	dev, err := t.HostDev()
	if err != nil {
		return err
	}
	if _, err := net.InterfaceByName(dev); err != nil {
		return fmt.Errorf("network %s: interface %s: %w", t.Name(), dev, err)
	}
	return nil
}
