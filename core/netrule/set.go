package netrule

import (
	"net/netip"
	"slices"
)

var (
	// anyV4 and anyV6 are every address of each family.
	anyV4 = netip.MustParsePrefix("0.0.0.0/0")
	anyV6 = netip.MustParsePrefix("::/0")

	// localhost is what systemd names so: the loopback addresses.
	localhost = []netip.Prefix{
		netip.MustParsePrefix("127.0.0.0/8"),
		netip.MustParsePrefix("::1/128"),
	}
)

// Any returns every address, of both families.
func Any() []netip.Prefix {
	return []netip.Prefix{anyV4, anyV6}
}

// Normalize returns the ranges of l as the smallest sorted list covering the
// same addresses: no range within another, and two halves of a range joined
// into it. An ipv4 range written as an ipv6 one is the ipv4 one.
func Normalize(l []netip.Prefix) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(l))
	for _, p := range l {
		if !p.IsValid() {
			continue
		}
		if p.Addr().Is4In6() && p.Bits() >= 96 {
			p = netip.PrefixFrom(p.Addr().Unmap(), p.Bits()-96)
		}
		out = append(out, p.Masked())
	}
	for {
		slices.SortFunc(out, comparePrefix)
		// A range sorts right after one holding it.
		kept := out[:0]
		for _, p := range out {
			if n := len(kept); n > 0 && contains(kept[n-1], p) {
				continue
			}
			kept = append(kept, p)
		}
		out = kept
		merged := false
		for i := 0; i+1 < len(out); i++ {
			if parent, ok := sibling(out[i], out[i+1]); ok {
				out[i] = parent
				out = slices.Delete(out, i+1, i+2)
				merged = true
			}
		}
		if !merged {
			return out
		}
	}
}

// comparePrefix orders the ipv4 ranges before the ipv6 ones, then by address,
// a range before the ones it holds.
func comparePrefix(a, b netip.Prefix) int {
	if a.Addr().Is4() != b.Addr().Is4() {
		if a.Addr().Is4() {
			return -1
		}
		return 1
	}
	if c := a.Addr().Compare(b.Addr()); c != 0 {
		return c
	}
	return a.Bits() - b.Bits()
}

// contains says every address of b is one of a.
func contains(a, b netip.Prefix) bool {
	return a.Addr().Is4() == b.Addr().Is4() && a.Bits() <= b.Bits() && a.Contains(b.Addr())
}

// sibling returns the range a and b are the two halves of, if they are.
func sibling(a, b netip.Prefix) (netip.Prefix, bool) {
	if a.Bits() != b.Bits() || a.Bits() == 0 || a.Addr().Is4() != b.Addr().Is4() {
		return netip.Prefix{}, false
	}
	parent := netip.PrefixFrom(a.Addr(), a.Bits()-1).Masked()
	if a != b && parent.Contains(b.Addr()) {
		return parent, true
	}
	return netip.Prefix{}, false
}

// halves returns the two halves of a range longer than one address.
func halves(p netip.Prefix) (netip.Prefix, netip.Prefix) {
	bits := p.Bits() + 1
	low := netip.PrefixFrom(p.Addr(), bits)
	b := p.Addr().AsSlice()
	b[(bits-1)/8] |= 1 << (7 - (bits-1)%8)
	addr, _ := netip.AddrFromSlice(b)
	return low, netip.PrefixFrom(addr, bits)
}

// Subtract returns the addresses of allow that are not in deny, as ranges: a
// denied range inside an allowed one splits it into the ranges around it.
func Subtract(allow, deny []netip.Prefix) []netip.Prefix {
	deny = Normalize(deny)
	out := make([]netip.Prefix, 0)
	var carve func(p netip.Prefix)
	carve = func(p netip.Prefix) {
		overlap := false
		for _, d := range deny {
			if contains(d, p) {
				return
			}
			if contains(p, d) {
				overlap = true
			}
		}
		if !overlap {
			out = append(out, p)
			return
		}
		low, high := halves(p)
		carve(low)
		carve(high)
	}
	for _, p := range Normalize(allow) {
		carve(p)
	}
	return Normalize(out)
}

// coversAny says l holds every address of both families.
func coversAny(l []netip.Prefix) bool {
	return slices.Contains(l, anyV4) && slices.Contains(l, anyV6)
}

// Strings returns the ranges as a backend takes them.
func Strings(l []netip.Prefix) []string {
	out := make([]string, len(l))
	for i, p := range l {
		out[i] = p.String()
	}
	return out
}

// ParsePrefixes reads the ranges a backend reports, an address being the
// range of itself.
func ParsePrefixes(l []string) ([]netip.Prefix, error) {
	out := make([]netip.Prefix, 0, len(l))
	for _, s := range l {
		p, err := parsePrefix(s)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return Normalize(out), nil
}
