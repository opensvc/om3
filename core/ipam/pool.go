package ipam

import (
	"bytes"
	"fmt"
	"math/big"
	"net"
	"sort"
	"strings"
)

// Pool is a span of addresses om hands out, from First to Last included.
type Pool struct {
	First net.IP
	Last  net.IP
}

// ParsePool reads a pool written as a subnet, as 192.168.10.128/26, or as its
// first and last addresses, as 192.168.10.100-192.168.10.199, the form
// nftables and ipset read.
func ParsePool(s string) (Pool, error) {
	if strings.Contains(s, "/") {
		_, ipnet, err := net.ParseCIDR(s)
		if err != nil {
			return Pool{}, err
		}
		return PoolOf(ipnet), nil
	}
	first, last, ok := strings.Cut(s, "-")
	if !ok {
		return Pool{}, fmt.Errorf("%q is neither a subnet, as 192.168.10.128/26, nor a span, as 192.168.10.100-192.168.10.199", s)
	}
	p := Pool{First: parseAddr(first), Last: parseAddr(last)}
	switch {
	case p.First == nil:
		return Pool{}, fmt.Errorf("%q: %q is not an ip address", s, first)
	case p.Last == nil:
		return Pool{}, fmt.Errorf("%q: %q is not an ip address", s, last)
	case (p.First.To4() == nil) != (p.Last.To4() == nil):
		return Pool{}, fmt.Errorf("%q: the first and last addresses are not of the same family", s)
	case bytes.Compare(p.First, p.Last) > 0:
		return Pool{}, fmt.Errorf("%q: the first address is past the last", s)
	}
	return p, nil
}

// parseAddr returns an address in its 4 bytes form when it is an ipv4 one, so
// two addresses compare byte by byte.
func parseAddr(s string) net.IP {
	ip := net.ParseIP(s)
	if v4 := ip.To4(); v4 != nil {
		return v4
	}
	return ip
}

// PoolOf returns the pool of all the addresses of a subnet.
func PoolOf(ipnet *net.IPNet) Pool {
	first := ipnet.IP.Mask(ipnet.Mask)
	last := make(net.IP, len(first))
	for i := range first {
		last[i] = first[i] | ^ipnet.Mask[i]
	}
	return Pool{First: first, Last: last}
}

// ParsePools reads the pools of a list, and refuses pools that overlap: an
// address handed out from two pools would be drawn twice as often as the
// others, and two pools written to overlap are a mistake.
func ParsePools(l []string) ([]Pool, error) {
	pools := make([]Pool, 0, len(l))
	for _, s := range l {
		p, err := ParsePool(s)
		if err != nil {
			return nil, err
		}
		pools = append(pools, p)
	}
	sorted := append([]Pool{}, pools...)
	sort.Slice(sorted, func(i, j int) bool { return ipToInt(sorted[i].First).Cmp(ipToInt(sorted[j].First)) < 0 })
	for i := 1; i < len(sorted); i++ {
		if sorted[i].Overlaps(sorted[i-1]) {
			return nil, fmt.Errorf("%s overlaps %s", sorted[i], sorted[i-1])
		}
	}
	return pools, nil
}

// Contains says ip is one of the addresses of the pool.
func (p Pool) Contains(ip net.IP) bool {
	if (ip.To4() == nil) != (p.First.To4() == nil) {
		return false
	}
	n := ipToInt(ip)
	return n.Cmp(ipToInt(p.First)) >= 0 && n.Cmp(ipToInt(p.Last)) <= 0
}

// Overlaps says the two pools have an address in common.
func (p Pool) Overlaps(o Pool) bool {
	return p.Contains(o.First) || p.Contains(o.Last) || o.Contains(p.First)
}

// In says every address of the pool is one of the subnet.
func (p Pool) In(ipnet *net.IPNet) bool {
	return ipnet.Contains(p.First) && ipnet.Contains(p.Last)
}

// Size returns the number of addresses of the pool.
func (p Pool) Size() *big.Int {
	n := new(big.Int).Sub(ipToInt(p.Last), ipToInt(p.First))
	return n.Add(n, big.NewInt(1))
}

// String returns the pool as it is written: a subnet when it is one, else its
// first and last addresses.
func (p Pool) String() string {
	if ipnet := p.subnet(); ipnet != nil {
		return ipnet.String()
	}
	return p.First.String() + "-" + p.Last.String()
}

// subnet returns the subnet the pool is all the addresses of, nil when it is
// not one.
func (p Pool) subnet() *net.IPNet {
	size := p.Size()
	bits := len(p.First) * 8
	host := size.BitLen() - 1
	if size.Cmp(new(big.Int).Lsh(big.NewInt(1), uint(host))) != 0 {
		return nil
	}
	ipnet := &net.IPNet{IP: p.First, Mask: net.CIDRMask(bits-host, bits)}
	if !p.First.Equal(p.First.Mask(ipnet.Mask)) {
		return nil
	}
	return ipnet
}

// PoolsString returns the pools as a list is written.
func PoolsString(pools []Pool) string {
	l := make([]string, len(pools))
	for i, p := range pools {
		l[i] = p.String()
	}
	return strings.Join(l, " ")
}
