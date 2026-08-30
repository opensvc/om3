package object

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/key"
)

// The node and cluster stores are not the core objects store, so a keyword
// declared there has to be declared here too. These are the ones the agent
// writes itself, or accepts in any section.
func TestNodeAndClusterKeywordLookup(t *testing.T) {
	cases := map[string]struct {
		store keywords.Store
		kind  naming.Kind
		key   key.T
	}{
		"the uuid the agent writes in node.conf": {
			store: NodeKeywordStore,
			kind:  naming.KindInvalid,
			key:   key.Parse("id"),
		},
		"the uuid the agent writes in cluster.conf": {
			store: ccfgKeywordStore,
			kind:  naming.KindCcfg,
			key:   key.Parse("id"),
		},
		"a node.conf DEFAULT comment": {
			store: NodeKeywordStore,
			kind:  naming.KindInvalid,
			key:   key.Parse("comment"),
		},
		"a node.conf section comment": {
			store: NodeKeywordStore,
			kind:  naming.KindInvalid,
			key:   key.Parse("node.comment"),
		},
		"a cluster.conf DEFAULT comment": {
			store: ccfgKeywordStore,
			kind:  naming.KindCcfg,
			key:   key.Parse("comment"),
		},
		"a cluster.conf driver section comment": {
			store: ccfgKeywordStore,
			kind:  naming.KindCcfg,
			key:   key.Parse("hb#1.comment"),
		},
	}
	for title, c := range cases {
		t.Run(title, func(t *testing.T) {
			require.NotNil(t, keywordLookup(c.store, c.key, c.kind, ""))
		})
	}
}
