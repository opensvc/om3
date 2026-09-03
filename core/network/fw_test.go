//go:build linux

package network

import (
	"slices"
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
		return slices.IndexFunc(legacyFWChains, func(data struct {
			Table string
			Chain string
		}) bool {
			return data.Chain == name
		})
	}
	jumper, target := index("osvc-postrouting"), index("osvc-masq")
	require.NotEqual(t, -1, jumper)
	require.NotEqual(t, -1, target)
	assert.Less(t, jumper, target, "the chain holding the jumps must be deleted first")
}
