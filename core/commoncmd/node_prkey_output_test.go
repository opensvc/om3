package commoncmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/daemon/api"
)

func nodeItems(names ...string) api.NodeItems {
	items := make(api.NodeItems, len(names))
	for i, name := range names {
		items[i].Meta.Node = name
	}
	return items
}

// The key of a single node is printed bare, as "om node config get --kw
// node.prkey --node" prints it; a selection of several nodes, or one that
// may select several, is a table naming each node (card 289).
func TestNodePRKeysOutput(t *testing.T) {
	bare := "tab=data.config.prkey"
	table := "tab=NAME:meta.node,PRKEY:data.config.prkey"
	assert.Equal(t, bare, nodePRKeysOutput("n2", nodeItems("n2")))
	assert.Equal(t, bare, nodePRKeysOutput("N2", nodeItems("n2")), "the name in another case")
	assert.Equal(t, table, nodePRKeysOutput("n*", nodeItems("n2")), "a glob")
	assert.Equal(t, table, nodePRKeysOutput("n1,n2", nodeItems("n1", "n2")))
	assert.Equal(t, table, nodePRKeysOutput("n2,n9", nodeItems("n2")), "a list of which one node exists")
	assert.Equal(t, table, nodePRKeysOutput("n2 n9", nodeItems("n2")), "a list separated by a unicode space")
	assert.Equal(t, table, nodePRKeysOutput("az=fr1", nodeItems("n2")), "a label")
}
