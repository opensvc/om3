//go:build linux

package network

import (
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// iptablesTables are the tables the iptables compatibility layer owns. A base
// chain om adds to one of them, on a hook and priority its standard chain
// already uses, makes the whole table unrepresentable in iptables terms, and
// every firewall driver going through iptables goes blind on it.
var iptablesTables = []string{"filter", "nat", "mangle", "raw", "security"}

// TestFWTableIsNotOneIptablesOwns pins the rule this module broke: om adds its
// rules to a table of its own.
//
// It used to add them to "nat" and "filter", which left "iptables -t nat -S"
// answering "table `nat' is incompatible, use 'nft' tool" and netavark failing
// every container start with "Chain already exists".
func TestFWTableIsNotOneIptablesOwns(t *testing.T) {
	assert.NotContainsf(t, iptablesTables, fwTableName,
		"%s is a table the iptables compatibility layer owns", fwTableName)
}

// TestLegacyFWChainsCoverTheOwnedChains pins that a setup cleans up every chain
// om used to leave elsewhere. A chain missing from the legacy list is one an
// upgraded node keeps in a table it no longer writes to, where it goes on
// hiding that table from iptables forever.
func TestLegacyFWChainsCoverTheOwnedChains(t *testing.T) {
	legacy := make([]string, 0, len(legacyFWChains))
	for _, data := range legacyFWChains {
		assert.Containsf(t, iptablesTables, data.Table,
			"%s is not a table om ever added a chain to", data.Table)
		legacy = append(legacy, data.Chain)
	}
	for _, chainName := range []string{fwChainPostrouting, fwChainMasq, fwChainForward} {
		assert.Containsf(t, legacy, chainName,
			"%s is not deleted from where om used to put it", chainName)
	}
}

// TestLegacyFWChainsDeleteTheJumperFirst pins the deletion order. nft refuses
// to delete a chain a rule still jumps to, and osvc-postrouting is what jumps
// to osvc-masq.
func TestLegacyFWChainsDeleteTheJumperFirst(t *testing.T) {
	index := func(name string) int {
		return slices.IndexFunc(legacyFWChains, func(data legacyChain) bool {
			return data.Chain == name
		})
	}
	jumper, target := index("osvc-postrouting"), index("osvc-masq")
	require.NotEqual(t, -1, jumper)
	require.NotEqual(t, -1, target)
	assert.Less(t, jumper, target, "the chain holding the jumps must be deleted first")
}

// TestFWRulesetIsOneTransaction pins the shape that makes a setup atomic: the
// table is created, deleted and defined again, all in one document.
//
// Adding a rule at a time left the masquerade and the forward accepts absent
// for as long as the setup ran.
func TestFWRulesetIsOneTransaction(t *testing.T) {
	got, err := fwRuleset([]fwNetwork{
		{CIDR: "10.100.0.0/22", Dev: "obr_backend3"},
		{CIDR: "127.0.0.1/32"},
		{CIDR: "fdfe::/112", Dev: "obr_backend1"},
	}, nil)
	require.NoError(t, err)

	assert.Equal(t, `table ip osvc { }
delete table ip osvc
table ip osvc {
	chain osvc-masq {
		ip daddr 10.100.0.0/22 counter return
		ip daddr 127.0.0.1/32 counter return
		ip daddr 224.0.0.0/8 counter return
		masquerade
	}
	chain osvc-postrouting {
		type nat hook postrouting priority srcnat; policy accept;
		ip saddr 10.100.0.0/22 counter jump osvc-masq
	}
	chain osvc-forward {
		type filter hook forward priority filter; policy accept;
		iif "obr_backend3" counter accept
		oif "obr_backend3" counter accept
	}
}
table ip6 osvc { }
delete table ip6 osvc
table ip6 osvc {
	chain osvc-masq {
		ip6 daddr fdfe::/112 counter return
		masquerade
	}
	chain osvc-postrouting {
		type nat hook postrouting priority srcnat; policy accept;
		ip6 saddr fdfe::/112 counter jump osvc-masq
	}
	chain osvc-forward {
		type filter hook forward priority filter; policy accept;
		iif "obr_backend1" counter accept
		oif "obr_backend1" counter accept
	}
}
`, got)
}

// TestFWRulesetSkipsAFamilyWithNoNetwork pins that no table is written for an
// address family nothing is configured in.
func TestFWRulesetSkipsAFamilyWithNoNetwork(t *testing.T) {
	got, err := fwRuleset([]fwNetwork{{CIDR: "10.22.0.0/16", Dev: "obr_default"}}, nil)
	require.NoError(t, err)
	assert.Contains(t, got, "table ip osvc {")
	assert.NotContains(t, got, "ip6 osvc")
}

// TestFWRulesetDeletesTheLegacyChainsFirst pins that a chain is emptied before
// any is deleted: nft refuses to delete a chain a rule still jumps to, and the
// jumps live in a chain of the same list.
func TestFWRulesetDeletesTheLegacyChainsFirst(t *testing.T) {
	got, err := fwRuleset(nil, []legacyChain{
		{Family: "ip", Table: "nat", Chain: "osvc-postrouting"},
		{Family: "ip", Table: "nat", Chain: "osvc-masq"},
	})
	require.NoError(t, err)
	assert.Equal(t, `flush chain ip nat osvc-postrouting
flush chain ip nat osvc-masq
delete chain ip nat osvc-postrouting
delete chain ip nat osvc-masq
`, got)
}

func TestFWRulesetRefusesABadCIDR(t *testing.T) {
	_, err := fwRuleset([]fwNetwork{{CIDR: "not-a-cidr"}}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not-a-cidr")
}

// TestFWRulesetIsAcceptedByNft parses the rendered document with the nft
// binary, which reports what it would refuse without applying anything.
func TestFWRulesetIsAcceptedByNft(t *testing.T) {
	if _, err := exec.LookPath("nft"); err != nil {
		t.Skip("nft is not installed")
	}
	ruleset, err := fwRuleset([]fwNetwork{
		{CIDR: "10.100.0.0/22", Dev: "obr_backend3"},
		{CIDR: "127.0.0.1/32"},
		{CIDR: "fdfe::/112", Dev: "obr_backend1"},
	}, nil)
	require.NoError(t, err)

	cmd := exec.Command("nft", "--check", "-f", "-")
	cmd.Stdin = strings.NewReader(ruleset)
	b, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "nft refused the ruleset: %s", b)
}
