package omon

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/placement"
	"github.com/opensvc/om3/v3/core/status"
)

func TestPlacementState(t *testing.T) {
	up := instance.Status{Avail: status.Up}
	down := instance.Status{Avail: status.Down}
	leader := instance.Monitor{IsLeader: true}
	other := instance.Monitor{}
	for name, test := range map[string]struct {
		avail    status.T
		monitors map[string]instance.Monitor
		statuses map[string]instance.Status
		expected placement.State
	}{
		"up on its leader": {
			avail:    status.Up,
			monitors: map[string]instance.Monitor{"n1": leader, "n2": other},
			statuses: map[string]instance.Status{"n1": up, "n2": down},
			expected: placement.Optimal,
		},
		"up on another node": {
			avail:    status.Up,
			monitors: map[string]instance.Monitor{"n1": leader, "n2": other},
			statuses: map[string]instance.Status{"n1": down, "n2": up},
			expected: placement.NonOptimal,
		},
		// Frozen everywhere, no instance is the ha leader, the daemon
		// moving nothing: the natural leader still says where the object
		// belongs.
		"frozen everywhere, up on another node": {
			avail:    status.Up,
			monitors: map[string]instance.Monitor{"n1": {IsLeader: true, IsHALeader: false}, "n2": other},
			statuses: map[string]instance.Status{"n1": down, "n2": up},
			expected: placement.NonOptimal,
		},
		"not up": {
			avail:    status.Down,
			monitors: map[string]instance.Monitor{"n1": leader, "n2": other},
			statuses: map[string]instance.Status{"n1": down, "n2": down},
			expected: placement.NotApplicable,
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expected, placementState(naming.KindSvc, test.avail, test.monitors, test.statuses))
		})
	}
	assert.Equal(t, placement.NotApplicable, placementState(naming.KindVol, status.Up, nil, nil), "only a service has a placement")
}
