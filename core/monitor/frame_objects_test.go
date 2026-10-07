package monitor

import (
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/topology"
)

func TestObjectWarning(t *testing.T) {
	InitColor()
	for name, test := range map[string]struct {
		status   *object.ActorStatus
		expected string
	}{
		"no actor status":        {},
		"up":                     {status: &object.ActorStatus{Avail: status.Up, Overall: status.Up, Topology: topology.Failover, UpInstancesCount: 1}},
		"overall warn":           {status: &object.ActorStatus{Avail: status.Up, Overall: status.Warn, Topology: topology.Failover, UpInstancesCount: 1}, expected: iconWarning},
		"failover up on 2 nodes": {status: &object.ActorStatus{Avail: status.Warn, Overall: status.Warn, Topology: topology.Failover, UpInstancesCount: 2}, expected: iconError},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expected, sObjectWarning(object.Status{ActorStatus: test.status}))
		})
	}
}

func TestObjectRunningIsRedOnExcessInstances(t *testing.T) {
	InitColor()
	color.NoColor = false
	defer func() { color.NoColor = true }()
	f := Frame{}
	f.Current.Cluster.Object = map[string]object.Status{
		"s1": {ActorStatus: &object.ActorStatus{Avail: status.Warn, Topology: topology.Failover, UpInstancesCount: 2}},
	}
	f.Current.Cluster.Node = map[string]node.Node{
		"n1": {Instance: map[string]instance.Instance{"s1": {Status: &instance.Status{Avail: status.Up}}}},
		"n2": {Instance: map[string]instance.Instance{"s1": {Status: &instance.Status{Avail: status.Up}}}},
	}
	assert.Equal(t, hired("2/1"), f.StrObjectRunning("s1"))
}
