package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
)

// TestConfigTargetFor pins the precedence that 'c' and 'e' both read.
//
// The selection handler sets viewPath from the row and viewNode from the
// column, independently, so an instance cell has both. Ordering the node
// case first is what made 'c' show the node configuration for a cell that
// names an object.
func TestConfigTargetFor(t *testing.T) {
	path, err := naming.ParsePath("ns1/svc/a")
	require.NoError(t, err)

	for name, tc := range map[string]struct {
		path     naming.Path
		node     string
		row, col int
		want     configTarget
	}{
		"an instance cell has both, and is about the object": {
			path: path, node: "node1", row: 4, col: 5, want: configTargetObject,
		},
		"an object name cell has only the path": {
			path: path, row: 4, col: 0, want: configTargetObject,
		},
		"a node header cell has only the node": {
			node: "node1", row: 0, col: 5, want: configTargetNode,
		},
		"the cluster cell has neither": {
			row: 0, col: 1, want: configTargetCluster,
		},
		"anything else is nothing": {
			row: 0, col: 3, want: configTargetNone,
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, configTargetFor(tc.path, tc.node, tc.row, tc.col))
		})
	}
}

// TestConfigTargetForPrefersTheObjectOverTheNode is the bug, on its own,
// so that a future reordering fails here rather than in a terminal.
func TestConfigTargetForPrefersTheObjectOverTheNode(t *testing.T) {
	path, err := naming.ParsePath("ns1/svc/a")
	require.NoError(t, err)
	assert.Equal(t, configTargetObject, configTargetFor(path, "node1", 7, 9),
		"a cell naming both an object and a node is about the object's configuration")
}
