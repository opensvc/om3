//go:build linux

package resipcni

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/opensvc/om3/v3/core/ipam"
	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/util/hostname"
)

// ipam returns the allocator of the network this resource plugs into, or nil
// when om allocates no address in it.
//
// A network keyword names a cni configuration, which om writes for the
// networks of the cluster and an administrator may write by hand for a plugin
// om knows nothing about. The second keeps the ipam of its own configuration:
// om allocates in the networks it owns, and leaves the others alone.
func (t *T) ipam() (*ipam.T, error) {
	nw, _, err := network.Lookup(t.Network)
	if err != nil {
		return nil, err
	}
	if nw == nil {
		return nil, nil
	}
	return network.NewAllocator(nw, hostname.Hostname())
}

// Configure reports a network this resource could never plug into, when the
// resource is loaded rather than when it is started.
//
// A name is good when a cni configuration of that name exists, whoever wrote
// it, or when it is a network of the cluster, whose configuration a network
// setup writes. A name that is neither is a renamed network or a typo.
func (t *T) Configure() error {
	if _, err := os.Stat(t.netConfFile()); err == nil {
		return nil
	}
	nw, names, err := network.Lookup(t.Network)
	if err != nil {
		return err
	}
	if nw != nil {
		return nil
	}
	return fmt.Errorf("unknown network %s: no %s, and no cluster network of that name, expected one of %s",
		t.Network, t.netConfFile(), strings.Join(names, ", "))
}

// ipamKey names the reservation of this resource. An instance holds as many ip
// resources as it needs, several of them in one network, so the address
// belongs to the resource rather than to the object.
func (t *T) ipamKey() string {
	return ipam.Key(t.Path, t.RID())
}

// allocateIP reserves the address of this resource, and returns the one it
// already holds when it holds one.
func (t *T) allocateIP() (net.IP, error) {
	i, err := t.ipam()
	if err != nil {
		return nil, err
	}
	if i == nil {
		return nil, nil
	}
	ip, err := i.Allocate(t.ipamKey())
	if err != nil {
		return nil, err
	}
	t.Log().Infof("allocated %s in network %s", ip, i.Name)
	return ip, nil
}

// freeIP releases the address of this resource.
func (t *T) freeIP() error {
	i, err := t.ipam()
	if err != nil {
		return err
	}
	if i == nil {
		return nil
	}
	return i.Free(t.ipamKey())
}

// staticIPAM rewrites the ipam section of a plugin configuration so the plugin
// is told the address rather than asked to pick one.
//
// The plugin still does the wiring, which is what a cni plugin is for. It no
// longer does the addressing, which om does for every network it owns, so one
// network can serve this driver and ip.netns without two allocators handing
// out the same address.
//
// The routes of the configuration are carried over untouched: they are the
// network's, and the static plugin takes the same ones. The gateway is the
// first address of the range, which is where both network drivers put their
// bridge, and which host-local defaulted to.
func staticIPAM(conf []byte, ip net.IP, rng *net.IPNet, gateway net.IP) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(conf, &m); err != nil {
		return nil, err
	}
	address := (&net.IPNet{IP: ip, Mask: rng.Mask}).String()
	static := map[string]any{
		"type":      "static",
		"addresses": []map[string]any{{"address": address, "gateway": gateway.String()}},
	}
	if previous, ok := m["ipam"].(map[string]any); ok {
		if routes, ok := previous["routes"]; ok {
			static["routes"] = routes
		}
	}
	m["ipam"] = static
	return json.Marshal(m)
}

// netConfBytesFor returns the configuration handed to the plugin, with the
// address om allocated written into it.
//
// A network om allocates no address in is handed its configuration as it is on
// disk, so a third party plugin keeps the ipam it was given.
func (t *T) netConfBytesFor(ip net.IP) ([]byte, error) {
	b, err := t.netConfBytes()
	if err != nil {
		return nil, err
	}
	if ip == nil {
		return b, nil
	}
	i, err := t.ipam()
	if err != nil {
		return nil, err
	}
	if i == nil || i.Range == nil {
		return b, nil
	}
	return staticIPAM(b, ip, i.Range, i.Gateway)
}

// allocatedIP returns the address reserved for this resource, or nil when it
// has none.
func (t *T) allocatedIP() (net.IP, error) {
	i, err := t.ipam()
	if err != nil {
		return nil, err
	}
	if i == nil {
		return nil, nil
	}
	return i.Allocated(t.ipamKey())
}
