package daemonapi

import (
	"context"
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

// The node monitor of the evicted node is a copy the heartbeats bring, behind
// it: an evict that follows the end of a drain waits for the copy to say so,
// on the events of its updates, rather than refuse the drain as running.
func TestWaitDrainEnd(t *testing.T) {
	const nodename = "node3"
	frozen := &node.Status{FrozenAt: time.Now()}
	setNode := func(t *testing.T, s *node.Status, m *node.Monitor) {
		node.StatusData.Set(nodename, s)
		node.MonitorData.Set(nodename, m)
		t.Cleanup(func() {
			node.StatusData.Unset(nodename)
			node.MonitorData.Unset(nodename)
		})
	}

	t.Run("accepts the node once an update says its drain ended", func(t *testing.T) {
		setNode(t, frozen, &node.Monitor{LocalExpect: node.MonitorLocalExpectDrained, State: node.MonitorStateDrainProgress})
		events := make(chan any)
		go func() {
			// An update not ending the drain, then the one ending it.
			events <- struct{}{}
			node.MonitorData.Set(nodename, &node.Monitor{State: node.MonitorStateIdle})
			events <- struct{}{}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		assert.Equal(t, "", waitDrainEnd(ctx, events, nodename))
		assert.NoError(t, ctx.Err(), "answered on the update, not at the deadline")
	})

	t.Run("refuses a drain no update says ended", func(t *testing.T) {
		setNode(t, frozen, &node.Monitor{State: node.MonitorStateDrainProgress})
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		assert.Equal(t, reasonDrainInProgress, waitDrainEnd(ctx, make(chan any), nodename))
	})

	t.Run("answers the other reasons at once", func(t *testing.T) {
		setNode(t, &node.Status{}, &node.Monitor{State: node.MonitorStateIdle})
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		assert.Contains(t, waitDrainEnd(ctx, make(chan any), nodename), "not frozen")
		assert.NoError(t, ctx.Err(), "not held until the deadline")
	})
}
