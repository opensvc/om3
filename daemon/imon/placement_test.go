package imon

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
)

// TestSpreadOrder pins the spread order to v2's, the md5 of the path and the
// nodename, which gives each object an order of its own. The expected orders
// are v2's, computed with python hashlib.
//
// The order used to be the nodenames sorted, the same for every object, so
// every object of a cluster led on its first node.
func TestSpreadOrder(t *testing.T) {
	nodes := []string{"dev2n1", "dev2n2", "dev2n3"}
	for s, want := range map[string][]string{
		"svc11":        {"dev2n2", "dev2n3", "dev2n1"},
		"svc11-1":      {"dev2n3", "dev2n1", "dev2n2"},
		"svc11-20":     {"dev2n1", "dev2n3", "dev2n2"},
		"test/svc/web": {"dev2n1", "dev2n2", "dev2n3"},
	} {
		p, err := naming.ParsePath(s)
		require.NoError(t, err)
		assert.Equal(t, want, spreadOrder(p, nodes), s)
	}
}

// TestShiftOrder pins that the slices of a scaler lead on successive nodes, as
// v2 rotated the nodes order by the slice index.
func TestShiftOrder(t *testing.T) {
	nodes := []string{"n1", "n2", "n3"}
	assert.Equal(t, nodes, shiftOrder(nodes, -1), "an object that is no slice")
	assert.Equal(t, nodes, shiftOrder(nodes, 0))
	assert.Equal(t, []string{"n2", "n3", "n1"}, shiftOrder(nodes, 1))
	assert.Equal(t, []string{"n3", "n1", "n2"}, shiftOrder(nodes, 2))
	assert.Equal(t, nodes, shiftOrder(nodes, 3))
	assert.Equal(t, []string{"n2", "n3", "n1"}, shiftOrder(nodes, 4))
	assert.Empty(t, shiftOrder(nil, 4))
}
