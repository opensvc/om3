package daemonapi

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// An object is known when the daemon lists it and an instance monitor
// watches it, on any node: a selection names what is listed, and an action is
// asked of a monitor. Either alone is the moment a provision run after a
// create was told the object did not exist.
func TestWaitObjectKnown(t *testing.T) {
	p := naming.Path{Kind: naming.KindSvc, Namespace: "test", Name: "known"}
	a := &DaemonAPI{localhost: "n1"}
	t.Cleanup(func() {
		object.StatusData.Unset(p)
		instance.MonitorData.Unset(p, "n2")
	})

	saved := configPropagationSafetyTick
	configPropagationSafetyTick = 10 * time.Millisecond
	t.Cleanup(func() { configPropagationSafetyTick = saved })

	wait := func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		// No event arrives: the safety tick is what looks again.
		return a.waitObjectKnown(ctx, &pubsub.Subscription{}, p)
	}

	require.False(t, wait(), "unknown")

	object.StatusData.Set(p, &object.Status{})
	require.False(t, wait(), "listed, and watched by no monitor yet")

	instance.MonitorData.Set(p, "n2", &instance.Monitor{})
	require.True(t, wait(), "listed, and watched on a peer")

	object.StatusData.Unset(p)
	require.False(t, wait(), "watched, and not listed")
}

// The creation of an object asked to wait holds as long as the wait for the
// daemon to know it, and objectKnownTimeout without: the instance monitor is
// published once the first status of the instance is in, which lasts as long
// as its drivers take, longer than objectKnownTimeout with many disks or a
// slow array.
func TestTheWaitBoundsTheHoldOfACreation(t *testing.T) {
	p := naming.Path{Kind: naming.KindSvc, Namespace: "test", Name: "slowstatus"}
	a := &DaemonAPI{localhost: "n1"}
	object.StatusData.Set(p, &object.Status{})
	t.Cleanup(func() {
		object.StatusData.Unset(p)
		instance.MonitorData.Unset(p, "n1")
	})

	savedTick := configPropagationSafetyTick
	configPropagationSafetyTick = 10 * time.Millisecond
	t.Cleanup(func() { configPropagationSafetyTick = savedTick })
	saved := objectKnownTimeout
	objectKnownTimeout = 100 * time.Millisecond
	t.Cleanup(func() { objectKnownTimeout = saved })

	// The first status of the instance ends, and the monitor is published,
	// past objectKnownTimeout.
	firstStatus := 300 * time.Millisecond
	hold := func(waiting bool) (bool, string) {
		instance.MonitorData.Unset(p, "n1")
		timer := time.AfterFunc(firstStatus, func() { instance.MonitorData.Set(p, "n1", &instance.Monitor{}) })
		defer timer.Stop()
		waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer waitCancel()
		ctx, cancel, bound := objectKnownContext(context.Background(), waitCtx, waiting)
		defer cancel()
		return a.waitObjectKnown(ctx, &pubsub.Subscription{}, p), bound
	}

	known, bound := hold(false)
	require.False(t, known, "with no wait, the hold ends at objectKnownTimeout")
	require.Equal(t, objectKnownTimeout.String(), bound)

	known, bound = hold(true)
	require.True(t, known, "with a wait, the hold lasts until the monitor is published")
	require.Equal(t, "the wait", bound)
}

// The hold of a creation asked with no wait is 30 seconds by default, longer
// than the first status of most instances.
func TestTheDefaultHoldOfACreation(t *testing.T) {
	require.Equal(t, 30*time.Second, objectKnownTimeout)
}
