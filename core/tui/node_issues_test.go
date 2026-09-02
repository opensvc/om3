package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/clusterdump"
	nodedata "github.com/opensvc/om3/v3/core/node"
)

// appWithNodeIssues returns an app whose dataset holds two nodes, one of
// them reporting a configuration issue.
func appWithNodeIssues() *App {
	a := NewApp(nil)
	a.Current = clusterdump.Data{
		Cluster: clusterdump.Cluster{
			Config: cluster.Config{Nodes: []string{"node1", "node2"}},
			Node: map[string]nodedata.Node{
				"node1": {Config: nodedata.Config{Issues: []string{"prkey 0xdead is also the prkey of node2"}}},
				"node2": {Config: nodedata.Config{}},
			},
		},
	}
	return a
}

// TestNodeIssuesRows pins what the mark on the states line opens: the
// issues of the node whose column was selected, and nothing for the
// node that has none.
func TestNodeIssuesRows(t *testing.T) {
	a := appWithNodeIssues()

	a.viewNodeIssues = "node1"
	assert.Equal(t, [][]string{{"node1", "prkey 0xdead is also the prkey of node2"}}, a.nodeIssuesRows())

	a.viewNodeIssues = "node2"
	assert.Empty(t, a.nodeIssuesRows(), "a node without issues lists nothing")

	a.viewNodeIssues = ""
	assert.Equal(t, [][]string{{"node1", "prkey 0xdead is also the prkey of node2"}}, a.nodeIssuesRows(),
		"every node is walked when none is named")
}

// TestNodeIssues covers the accessor the states line mark and the view
// share.
func TestNodeIssues(t *testing.T) {
	a := appWithNodeIssues()
	assert.Len(t, a.nodeIssues("node1"), 1)
	assert.Empty(t, a.nodeIssues("node2"))
	assert.Empty(t, a.nodeIssues("nosuchnode"), "an unknown node has no issues to report")
}
