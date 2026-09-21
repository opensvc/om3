package nodeselector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/nodesinfo"
)

// The nodes a daemon knows are held in a map, which hands them out in a
// different order every time it is walked. A selector expanding to all of them
// has to answer the same thing twice all the same: whoever compares two
// evaluations of a "nodes = *" reads a change where the order alone differs.
func TestExpandIsStable(t *testing.T) {
	info := nodesinfo.M{
		"n3": nodesinfo.T{},
		"n1": nodesinfo.T{},
		"n2": nodesinfo.T{},
		"n4": nodesinfo.T{},
		"n5": nodesinfo.T{},
	}
	want := []string{"n1", "n2", "n3", "n4", "n5"}
	for i := 0; i < 10; i++ {
		l, err := New("*", WithNodesInfo(info)).Expand()
		require.NoError(t, err)
		assert.Equal(t, want, l)
	}
}
