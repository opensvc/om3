package monitor

import (
	"testing"

	"github.com/stretchr/testify/assert"

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
