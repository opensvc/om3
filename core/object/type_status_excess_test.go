package object

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/topology"
)

func TestExcessInstances(t *testing.T) {
	for _, c := range []struct {
		name   string
		status *ActorStatus
		want   int
	}{
		{"no status", nil, 0},
		{"failover, one up", &ActorStatus{Topology: topology.Failover, UpInstancesCount: 1}, 0},
		{"failover, two up", &ActorStatus{Topology: topology.Failover, UpInstancesCount: 2}, 1},
		{"flex, up to max", &ActorStatus{Topology: topology.Flex, UpInstancesCount: 3, Flex: &FlexStatus{Max: 3}}, 0},
		{"flex, beyond max", &ActorStatus{Topology: topology.Flex, UpInstancesCount: 4, Flex: &FlexStatus{Max: 3}}, 1},
		{"flex, bounds not known", &ActorStatus{Topology: topology.Flex, UpInstancesCount: 4}, 0},
	} {
		assert.Equal(t, c.want, c.status.ExcessInstances(), c.name)
	}
}

func TestExpectedInstances(t *testing.T) {
	assert.Equal(t, 0, (*ActorStatus)(nil).ExpectedInstances())
	assert.Equal(t, 1, (&ActorStatus{Topology: topology.Failover}).ExpectedInstances())
	assert.Equal(t, 3, (&ActorStatus{Topology: topology.Flex, Flex: &FlexStatus{Target: 3, Max: 5}}).ExpectedInstances())
}
