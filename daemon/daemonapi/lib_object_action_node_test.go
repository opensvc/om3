package daemonapi

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/instance"
)

// A node without an instance hands the requests for an orchestration to the
// same instance node, the first in name order it hears, for the requests in
// a row to be judged by the node that saw the one before clear.
func TestActionNode(t *testing.T) {
	monitors := map[string]*instance.Monitor{"n3": {}, "n2": {}, "n4": {}}
	hearsAll := func(string, string) bool { return true }
	hearsN4 := func(_, nodename string) bool { return nodename == "n4" }
	hearsNone := func(string, string) bool { return false }
	for range 20 {
		assert.Equal(t, "n2", actionNode("n1", monitors, hearsAll), "map order does not matter")
	}
	assert.Equal(t, "n4", actionNode("n1", monitors, hearsN4), "the first heard")
	assert.Equal(t, "n2", actionNode("n1", monitors, hearsNone), "none heard: the first")
	assert.Equal(t, "", actionNode("n1", map[string]*instance.Monitor{"n1": {}}, hearsAll), "no instance elsewhere")
}
