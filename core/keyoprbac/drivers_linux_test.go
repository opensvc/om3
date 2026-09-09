//go:build linux

package keyoprbac

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/drivers/resipcni"
	"github.com/opensvc/om3/v3/drivers/resipnetns"
)

// sectionOptions returns every keyword a section of this driver may carry: the
// ones the driver defines, and the ones om adds to every section of an object
// configuration.
//
// Reading them from the driver and from the object keyword store, rather than
// from a list written here, is what makes this a guard: a keyword added to
// either shows up here on its own.
func sectionOptions(r resource.Driver) []string {
	l := make([]string, 0)
	for _, kw := range r.Manifest().Keywords() {
		l = append(l, kw.Option)
	}
	for _, kw := range object.KeywordStoreWithDrivers(naming.KindSvc) {
		if kw.Section == "" {
			l = append(l, kw.Option)
		}
	}
	return l
}

// TestIPCNIKeepsEveryKeywordItHad guards the group default of the ip group.
//
// Giving a group a default refuses the keywords the policy has not weighed,
// which is the point, but it must not refuse the keywords every section
// carries: a comment, a process group limit, a subset. Those were writable in
// every group before the default existed, and an ip.cni resource is the proof,
// because every keyword of its own driver is open.
//
// This test reads the driver rather than a list written here, so a keyword
// added to the common set, or to ip.cni, fails here instead of silently
// becoming root-only for every user.
func TestIPCNIKeepsEveryKeywordItHad(t *testing.T) {
	for _, option := range sectionOptions(resipcni.New()) {
		if option == "type" {
			// The type has a rule of its own, tested elsewhere.
			continue
		}
		if triggers[option] {
			assert.Errorf(t, Denied(noGrant, "ip#1", option, "x", section("network")),
				"trigger %s must stay refused", option)
			continue
		}
		assert.NoErrorf(t, Denied(noGrant, "ip#1", option, "x", section("network")),
			"ip.cni keyword %s was writable before the ip group had a default", option)
	}
}

// TestIPNetnsAddressKeywordsAreRefused pins the other half: the keywords of
// ip.netns that name an address, or the link of the node that carries it, are
// the ones the group default exists for.
func TestIPNetnsAddressKeywordsAreRefused(t *testing.T) {
	refused := map[string]bool{
		"name": true, "dev": true, "gateway": true, "netmask": true,
		"macaddr": true, "mode": true, "alias": true, "vlan_tag": true,
		"vlan_mode": true, "del_net_route": true, "check_carrier": true,
	}
	seen := make(map[string]bool)
	for _, option := range sectionOptions(resipnetns.New()) {
		if !refused[option] {
			continue
		}
		seen[option] = true
		assert.Errorf(t, Denied(noGrant, "ip#1", option, "x", section("network")),
			"ip.netns keyword %s decides an address or a link of the node", option)
	}
	for option := range refused {
		assert.Truef(t, seen[option], "ip.netns no longer has a %s keyword: is the rule still right?", option)
	}
}
