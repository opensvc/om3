package resiprule

import (
	"strings"
	"testing"
)

// A rule selecting an IPv6 source or destination is an IPv6 rule, whatever
// the other words of the specification.
func TestIsIPv6Spec(t *testing.T) {
	for spec, want := range map[string]bool{
		"from 192.168.100.0/24 table 100":     false,
		"to 10.0.0.1 lookup main":             false,
		"from all fwmark 0x1 lookup 100":      false,
		"from 2001:db8::/32 table 100":        true,
		"pref 100 to fd00::1 lookup 200":      true,
		"iif eth0 from 2001:db8::1 table 100": true,
		"from all to 2001:db8::/64 prohibit":  true,
		"from":                                false,
	} {
		if got := isIPv6Spec(strings.Fields(spec)); got != want {
			t.Errorf("%q: got %v, want %v", spec, got, want)
		}
	}
}
