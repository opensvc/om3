package netrule

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
)

func prefixes(l ...string) []netip.Prefix {
	out := make([]netip.Prefix, len(l))
	for i, s := range l {
		out[i] = netip.MustParsePrefix(s)
	}
	return out
}

func TestNormalize(t *testing.T) {
	assert.Equal(t, []string{"10.0.0.0/8", "192.168.0.0/23", "fd00::/8"},
		Strings(Normalize(prefixes("192.168.1.0/24", "fd00::/8", "10.1.0.0/16", "10.0.0.0/8", "192.168.0.0/24", "fd00:1::/32", "10.0.0.0/8"))),
		"sorted, the ranges within another dropped, two halves joined")
	assert.Equal(t, []string{"0.0.0.0/0"}, Strings(Normalize(prefixes("0.0.0.0/1", "128.0.0.0/1"))))
	assert.Equal(t, []string{"10.0.0.0/30"}, Strings(Normalize(prefixes("10.0.0.0/32", "10.0.0.1/32", "10.0.0.2/32", "10.0.0.3/32"))),
		"joined again once joined")
	assert.Equal(t, []string{"10.0.0.1/32"}, Strings(Normalize(prefixes("::ffff:10.0.0.1/128"))),
		"an ipv4 address written as an ipv6 one is the ipv4 one")
	assert.Empty(t, Normalize(nil))
}

func TestSubtract(t *testing.T) {
	cases := []struct {
		name  string
		allow []string
		deny  []string
		want  []string
	}{
		{"nothing denied", []string{"10.0.0.0/24"}, nil, []string{"10.0.0.0/24"}},
		{"denied elsewhere", []string{"10.0.0.0/24"}, []string{"10.0.1.0/24", "fd00::/8"}, []string{"10.0.0.0/24"}},
		{"all denied", []string{"10.0.0.0/24"}, []string{"10.0.0.0/8"}, []string{}},
		{"the same range", []string{"10.0.0.0/24"}, []string{"10.0.0.0/24"}, []string{}},
		{"a host of a range", []string{"10.0.0.0/30"}, []string{"10.0.0.1/32"}, []string{"10.0.0.0/32", "10.0.0.2/31"}},
		{"a half", []string{"10.0.0.0/24"}, []string{"10.0.0.128/25"}, []string{"10.0.0.0/25"}},
		{"two holes", []string{"10.0.0.0/29"}, []string{"10.0.0.0/32", "10.0.0.7/32"}, []string{"10.0.0.1/32", "10.0.0.2/31", "10.0.0.4/31", "10.0.0.6/32"}},
		{"ipv6", []string{"fd00::/126"}, []string{"fd00::2/128"}, []string{"fd00::/127", "fd00::3/128"}},
		{"each family apart", []string{"10.0.0.0/31", "fd00::/127"}, []string{"10.0.0.0/32", "fd00::1/128"}, []string{"10.0.0.1/32", "fd00::/128"}},
		{"any but a host", []string{"0.0.0.0/0"}, []string{"128.0.0.0/1", "0.0.0.0/2"}, []string{"64.0.0.0/2"}},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, Strings(Subtract(prefixes(c.allow...), prefixes(c.deny...))), c.name)
	}
}

func TestSubtractAHostOfAnyLeavesEveryOtherAddress(t *testing.T) {
	left := Subtract(Any(), prefixes("10.29.0.13/32"))
	// 32 ranges of ipv4, one per bit of the address, and all of ipv6.
	assert.Len(t, left, 33)
	denied := netip.MustParseAddr("10.29.0.13")
	for _, addr := range []string{"10.29.0.12", "10.29.0.14", "0.0.0.0", "255.255.255.255", "::1"} {
		found := false
		for _, p := range left {
			assert.False(t, p.Contains(denied))
			found = found || p.Contains(netip.MustParseAddr(addr))
		}
		assert.True(t, found, addr)
	}
}

func TestParsePrefixes(t *testing.T) {
	l, err := ParsePrefixes([]string{"::1/128", "127.0.0.0/8", "10.29.0.12"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"10.29.0.12/32", "127.0.0.0/8", "::1/128"}, Strings(l))
	_, err = ParsePrefixes([]string{"localhost"})
	assert.Error(t, err)
}
