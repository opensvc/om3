package resvol

import (
	"fmt"
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

// A volume named per node, as {name}-{nodename}, is a volume of its own on
// each node of a flex object: it keeps its once access, and gets no warning.
func TestNamePerNode(t *testing.T) {
	eval := func(names map[string]string) func(string) (string, error) {
		return func(n string) (string, error) {
			if v, ok := names[n]; ok {
				return v, nil
			}
			return "", fmt.Errorf("%s: not evaluated", n)
		}
	}
	assert.True(t, namePerNode("web-n1", []string{"n2", "n3"}, eval(map[string]string{"n2": "web-n2", "n3": "web-n3"})))
	assert.False(t, namePerNode("web", []string{"n2"}, eval(map[string]string{"n2": "web"})), "a shared name")
	assert.False(t, namePerNode("web-n1", []string{"n2", "n3"}, eval(map[string]string{"n2": "web-n2", "n3": "web-n1"})), "another node evaluating the same")
	assert.False(t, namePerNode("web-n1", []string{"n2"}, eval(map[string]string{})), "a name not evaluated")
	assert.False(t, namePerNode("web-n1", nil, eval(map[string]string{})), "an object of one node")
	assert.False(t, namePerNode("", []string{"n2"}, eval(map[string]string{"n2": "web-n2"})), "no name")
}
