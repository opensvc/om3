package network

import (
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strings"

	"github.com/opensvc/om3/v3/core/ipam"
)

// ReverseZones returns the reverse zones holding the PTR records of the
// addresses om hands out from the network, with no trailing dot: the ones of
// the ranges of a network handing out some of its segment only, as a lan
// network, else the ones of the whole network.
//
// A zone is cut at an octet boundary for ipv4 and at a nibble boundary for
// ipv6, the finest covering the addresses and no others: the zones the
// cluster answers for hold none of the addresses it does not hand out, as
// the ones of the nodes, which the cluster nameservers would otherwise say
// do not exist. A /22 is four /24 zones, a /16 one, a range of a few
// addresses a zone each.
//
// The lo network hands out no address, and has none.
func ReverseZones(nw Networker) ([]string, error) {
	if nw.Type() == "lo" {
		return nil, nil
	}
	var pools []ipam.Pool
	if i, ok := nw.(Pooler); ok {
		l, err := i.Pools()
		if err != nil {
			return nil, err
		}
		pools = l
	} else {
		_, ipnet, err := net.ParseCIDR(nw.Network())
		if err != nil {
			return nil, nil
		}
		pools = []ipam.Pool{ipam.PoolOf(ipnet)}
	}
	l := make([]string, 0)
	for _, pool := range pools {
		first, ok1 := netip.AddrFromSlice(pool.First)
		last, ok2 := netip.AddrFromSlice(pool.Last)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("network %s: invalid range %s-%s", nw.Name(), pool.First, pool.Last)
		}
		for _, prefix := range rangePrefixes(first.Unmap(), last.Unmap()) {
			for _, zone := range prefixZones(prefix) {
				if !slices.Contains(l, zone) {
					l = append(l, zone)
				}
			}
		}
	}
	return l, nil
}

// rangePrefixes returns the fewest prefixes covering the addresses from first
// to last, in order.
func rangePrefixes(first, last netip.Addr) []netip.Prefix {
	l := make([]netip.Prefix, 0)
	if first.BitLen() != last.BitLen() || last.Less(first) {
		return l
	}
	bits := first.BitLen()
	for addr := first; ; {
		// The shortest prefix starting at addr and ending at last or before.
		size := bits
		for size > 0 {
			p := netip.PrefixFrom(addr, size-1).Masked()
			if p.Addr() != addr || lastAddr(p).Compare(last) > 0 {
				break
			}
			size--
		}
		p := netip.PrefixFrom(addr, size)
		l = append(l, p)
		end := lastAddr(p)
		if end.Compare(last) >= 0 {
			return l
		}
		addr = end.Next()
	}
}

// lastAddr returns the last address of the prefix.
func lastAddr(p netip.Prefix) netip.Addr {
	b := p.Masked().Addr().AsSlice()
	for i := p.Bits(); i < len(b)*8; i++ {
		b[i/8] |= 1 << (7 - i%8)
	}
	a, _ := netip.AddrFromSlice(b)
	return a
}

// prefixZones returns the reverse zones of the prefix: the ones of its
// subprefixes at the next octet boundary for ipv4, nibble boundary for ipv6,
// at its own when it is on one.
func prefixZones(p netip.Prefix) []string {
	step := 8
	if p.Addr().Is6() {
		step = 4
	}
	bits := (p.Bits() + step - 1) / step * step
	if bits == 0 {
		bits = step
	}
	l := make([]string, 0, 1<<(bits-p.Bits()))
	for addr := p.Masked().Addr(); p.Contains(addr); {
		l = append(l, reverseZoneName(addr, bits))
		next := lastAddr(netip.PrefixFrom(addr, bits)).Next()
		if !next.IsValid() || !p.Contains(next) {
			break
		}
		addr = next
	}
	return l
}

// reverseZoneName returns the reverse zone of the first bits of addr, which
// are on an octet boundary for ipv4, a nibble one for ipv6.
func reverseZoneName(addr netip.Addr, bits int) string {
	b := addr.AsSlice()
	labels := make([]string, 0, bits/4)
	if addr.Is4() {
		for i := 0; i < bits/8; i++ {
			labels = append(labels, fmt.Sprint(b[i]))
		}
		slices.Reverse(labels)
		return strings.Join(append(labels, "in-addr.arpa"), ".")
	}
	for i := 0; i < bits/4; i++ {
		v := b[i/2]
		if i%2 == 0 {
			v >>= 4
		}
		labels = append(labels, fmt.Sprintf("%x", v&0xf))
	}
	slices.Reverse(labels)
	return strings.Join(append(labels, "ip6.arpa"), ".")
}

// InReverseZones says whether the reverse name, as a PTR record names it, is
// in one of the zones.
func InReverseZones(name string, zones []string) bool {
	name = strings.TrimSuffix(name, ".")
	for _, zone := range zones {
		if name == zone || strings.HasSuffix(name, "."+zone) {
			return true
		}
	}
	return false
}
