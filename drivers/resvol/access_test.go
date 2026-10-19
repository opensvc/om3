package resvol

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/topology"
)

// A flex object starts its volume on each of its nodes, so the volume is
// created open to several nodes: rwo becomes rwx, roo becomes rox.
func TestAccess(t *testing.T) {
	for _, c := range []struct {
		access string
		topo   topology.T
		want   string
	}{
		{"rwo", topology.Failover, "rwo"},
		{"rwo", topology.Flex, "rwx"},
		{"roo", topology.Flex, "rox"},
		{"rwx", topology.Failover, "rwx"},
	} {
		r := T{Access: c.access, Topology: c.topo}
		a, err := r.access()
		assert.NoError(t, err)
		assert.Equal(t, c.want, a.String(), "%s %s", c.access, c.topo)
	}
	_, err := (&T{Access: "bad"}).access()
	assert.Error(t, err)
}
