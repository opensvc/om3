package daemonapi

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/status"
)

func TestNotDrainedReason(t *testing.T) {
	const nodename = "node2"

	frozen := &node.Status{FrozenAt: time.Now()}
	unfrozen := &node.Status{}

	svc1 := naming.Path{Name: "svc1", Kind: naming.KindSvc}
	// A drain shuts down the svc instances only, so a vol left up is not what
	// keeps a node from being evicted.
	vol1 := naming.Path{Name: "vol1", Kind: naming.KindVol}

	setInstance := func(t *testing.T, p naming.Path, avail status.T) {
		instance.StatusData.Set(p, nodename, &instance.Status{Avail: avail})
		t.Cleanup(func() {
			instance.StatusData.Unset(p, nodename)
		})
	}
	setNode := func(t *testing.T, s *node.Status, m *node.Monitor) {
		node.StatusData.Set(nodename, s)
		t.Cleanup(func() {
			node.StatusData.Unset(nodename)
		})
		if m != nil {
			node.MonitorData.Set(nodename, m)
			t.Cleanup(func() {
				node.MonitorData.Unset(nodename)
			})
		}
	}

	t.Run("accepts a frozen node running nothing", func(t *testing.T) {
		setNode(t, frozen, &node.Monitor{State: node.MonitorStateIdle})
		setInstance(t, svc1, status.Down)

		assert.Equal(t, "", notDrainedReason(nodename))
	})

	t.Run("accepts a frozen node whose drain had no instance to stop", func(t *testing.T) {
		setNode(t, frozen, &node.Monitor{State: node.MonitorStateIdle})

		assert.Equal(t, "", notDrainedReason(nodename))
	})

	t.Run("ignores a kind the drain does not shut down", func(t *testing.T) {
		setNode(t, frozen, &node.Monitor{State: node.MonitorStateIdle})
		setInstance(t, vol1, status.Up)

		assert.Equal(t, "", notDrainedReason(nodename))
	})

	t.Run("refuses a node it knows nothing about", func(t *testing.T) {
		assert.Contains(t, notDrainedReason("node-never-seen"), "unknown")
	})

	t.Run("refuses an unfrozen node", func(t *testing.T) {
		setNode(t, unfrozen, &node.Monitor{State: node.MonitorStateIdle})

		assert.Contains(t, notDrainedReason(nodename), "not frozen")
	})

	t.Run("refuses a node whose drain is still running", func(t *testing.T) {
		for name, mon := range map[string]*node.Monitor{
			"local expect drained": {LocalExpect: node.MonitorLocalExpectDrained, State: node.MonitorStateIdle},
			"draining":             {State: node.MonitorStateDrainProgress},
		} {
			t.Run(name, func(t *testing.T) {
				setNode(t, frozen, mon)

				assert.Contains(t, notDrainedReason(nodename), "drain is in progress")
			})
		}
	})

	t.Run("refuses a node whose last drain failed", func(t *testing.T) {
		setNode(t, frozen, &node.Monitor{State: node.MonitorStateDrainFailure})

		assert.Contains(t, notDrainedReason(nodename), "last drain failed")
	})

	t.Run("refuses a frozen node still running an instance", func(t *testing.T) {
		for name, avail := range map[string]status.T{
			"up":                 status.Up,
			"warn":               status.Warn,
			"standby up with up": status.StandbyUpWithUp,
		} {
			t.Run(name, func(t *testing.T) {
				setNode(t, frozen, &node.Monitor{State: node.MonitorStateIdle})
				setInstance(t, svc1, avail)

				assert.Contains(t, notDrainedReason(nodename), "still runs "+svc1.String())
			})
		}
	})
}
