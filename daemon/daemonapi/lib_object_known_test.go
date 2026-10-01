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
