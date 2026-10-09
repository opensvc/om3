// Package networklan is the lan network driver: a range of the segment the
// nodes share, whose addresses om hands out to the cluster rather than to a
// node, for an object to take its address along to the node it moves to, as
// the floating address of a failover service.
//
// Nothing is set up on the nodes: the addresses are configured on an
// interface of the node on the segment, which is reached as the rest of the
// segment is, and leave it as they are, the hosts of the segment answering
// them directly.
package networklan

import (
	"fmt"
	"net"
	"strconv"

	"github.com/opensvc/om3/v3/core/driver"
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

// AllocatableRange returns the whole range, whatever the node: every node
// draws from it, so an object draws the same address wherever it runs.
func (t *T) AllocatableRange(_ string) (*net.IPNet, error) {
	return t.IPNet()
}

// IsClusterWide says the addresses of a lan network belong to the cluster.
func (t *T) IsClusterWide() bool {
	return true
}

// Netmask returns the prefix length of the segment, which holds the range:
// the netmask keyword, or else the prefix length of the address of this node
// on the segment, the one whose prefix holds the range.
func (t *T) Netmask() (int, error) {
	rng, err := t.IPNet()
	if err != nil {
		return 0, err
	}
	ones, _ := rng.Mask.Size()
	if s := t.GetString("netmask"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			return 0, fmt.Errorf("network#%s.netmask %q: %w", t.Name(), s, err)
		}
		if n < 0 || n > ones {
			return 0, fmt.Errorf("network#%s.netmask %d: the segment must hold the range %s, so its prefix length is at most %d", t.Name(), n, rng, ones)
		}
		return n, nil
	}
	_, n, err := t.hostAddr()
	return n, err
}

// HostDev returns the interface of this node on the segment: the dev keyword,
// or else the interface holding the address of this node on the segment.
func (t *T) HostDev() (string, error) {
	if s := t.GetString("dev"); s != "" {
		return s, nil
	}
	dev, _, err := t.hostAddr()
	return dev, err
}

// hostAddr returns the interface and the prefix length of the address of this
// node on the segment: the address whose prefix holds the range, on the
// interface the dev keyword names when it names one. The deepest prefix wins,
// as the route to it does.
//
// A node with no address on the segment, as on a segment dedicated to the
// addresses of the services, has neither: the netmask keyword, and the dev
// keyword, say them then.
func (t *T) hostAddr() (string, int, error) {
	rng, err := t.IPNet()
	if err != nil {
		return "", 0, err
	}
	rngOnes, _ := rng.Mask.Size()
	devKw := t.GetString("dev")
	l, err := interfaces()
	if err != nil {
		return "", 0, err
	}
	var (
		dev  string
		ones = -1
		tie  bool
	)
	for _, i := range l {
		if i.Flags&net.FlagLoopback != 0 || (devKw != "" && i.Name != devKw) {
			continue
		}
		addrs, err := addrsOf(i)
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			_, prefix, err := net.ParseCIDR(addr.String())
			if err != nil {
				continue
			}
			n, _ := prefix.Mask.Size()
			if n > rngOnes || !prefix.Contains(rng.IP) {
				continue
			}
			switch {
			case n > ones:
				dev, ones, tie = i.Name, n, false
			case n == ones && i.Name != dev:
				tie = true
			}
		}
	}
	switch {
	case ones < 0 && devKw != "":
		return "", 0, fmt.Errorf("no address of interface %s holds the range %s of the network %s, so the prefix length of its segment is unknown: set the netmask keyword of the network", devKw, rng, t.Name())
	case ones < 0:
		return "", 0, fmt.Errorf("no address of this node holds the range %s of the network %s, so neither the interface nor the prefix length of its segment is known: set the dev and netmask keywords of the network", rng, t.Name())
	case tie:
		return "", 0, fmt.Errorf("several interfaces of this node hold an address of a /%d prefix holding the range %s of the network %s: set the dev keyword of the network", ones, rng, t.Name())
	}
	return dev, ones, nil
}

// Setup checks the node is on the segment, which is all a lan network needs
// of it.
func (t *T) Setup() error {
	dev, err := t.HostDev()
	if err != nil {
		return err
	}
	if _, err := t.Netmask(); err != nil {
		return err
	}
	if _, err := net.InterfaceByName(dev); err != nil {
		return fmt.Errorf("network %s: interface %s: %w", t.Name(), dev, err)
	}
	return nil
}
