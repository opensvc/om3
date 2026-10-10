package network

import (
	"fmt"
	"net"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/ipam"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/hostname"
)

// ClusterWideAllocators returns the allocators of this node in the networks
// every node draws from.
func ClusterWideAllocators(n *object.Node) []*ipam.T {
	l := make([]*ipam.T, 0)
	for _, nw := range Networks(n) {
		i, err := NewAllocator(nw, hostname.Hostname())
		if err != nil || i == nil || !i.ClusterWide || i.Range == nil {
			continue
		}
		l = append(l, i)
	}
	return l
}

// InstanceAddrs returns the addresses an instance status reports holding in
// the ranges of the allocators, by network name, each the key of the
// resource reporting it.
func InstanceAddrs(allocators []*ipam.T, p naming.Path, st instance.Status) map[string]map[string]string {
	m := make(map[string]map[string]string, len(allocators))
	for _, i := range allocators {
		m[i.Name] = make(map[string]string)
	}
	for rid, rstat := range st.Resources {
		v, ok := rstat.Info[ipAddrInfoKey]
		if !ok || v == nil {
			continue
		}
		ip := net.ParseIP(fmt.Sprint(v))
		if ip == nil {
			continue
		}
		for _, i := range allocators {
			if i.Range.Contains(ip) {
				m[i.Name][ip.String()] = ipam.Key(p, rid)
			}
		}
	}
	return m
}
