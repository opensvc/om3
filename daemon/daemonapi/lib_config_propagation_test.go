package daemonapi

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// propagationFixture holds the configurations the nodes of p have, and the
// nodes the cluster data holds, as the heartbeats would have made them.
func propagationFixture(t *testing.T, p naming.Path, live []string) {
	t.Helper()
	for _, nodename := range live {
		node.MonitorData.Set(nodename, &node.Monitor{})
	}
	t.Cleanup(func() {
		for _, nodename := range []string{"n1", "n2", "n3"} {
			node.MonitorData.Unset(nodename)
			instance.ConfigData.Unset(p, nodename)
		}
	})
}

func setConfig(p naming.Path, nodename string, at time.Time, scope ...string) {
	instance.ConfigData.Set(p, nodename, &instance.Config{Path: p, UpdatedAt: at, Scope: scope})
}

// A configuration has propagated when every live node of the scope holds it,
// or one written after it.
func TestConfigLaggards(t *testing.T) {
	p := naming.Path{Kind: naming.KindSvc, Name: "propagated"}
	a := &DaemonAPI{localhost: "n1"}
	written := time.Now()
	before := written.Add(-time.Minute)

	propagationFixture(t, p, []string{"n1", "n2", "n3"})

	setConfig(p, "n1", before, "n1", "n2")
	assert.Equal(t, []string{"n1"}, a.configLaggards(p, written),
		"until this node read back what it wrote, the scope written is not known")

	setConfig(p, "n1", written, "n1", "n2", "n3")
	setConfig(p, "n2", written, "n1", "n2", "n3")
	assert.Equal(t, []string{"n3"}, a.configLaggards(p, written),
		"a node the configuration adds to the scope receives it too")

	setConfig(p, "n3", before, "n1", "n2")
	assert.Equal(t, []string{"n3"}, a.configLaggards(p, written), "n3 still holds the configuration replaced")

	setConfig(p, "n3", written.Add(time.Second), "n1", "n2", "n3")
	assert.Empty(t, a.configLaggards(p, written), "a configuration written since is at least as new")
}

// A node whose data the cluster dropped, its heartbeats stale, is not alive,
// and fetches the configuration when it comes back: it is not waited for.
func TestConfigLaggardsSkipsANodeThatIsNotAlive(t *testing.T) {
	p := naming.Path{Kind: naming.KindSvc, Name: "propagated"}
	a := &DaemonAPI{localhost: "n1"}
	written := time.Now()

	propagationFixture(t, p, []string{"n1", "n2"})
	setConfig(p, "n1", written, "n1", "n2", "n3")
	setConfig(p, "n2", written, "n1", "n2", "n3")
	assert.Empty(t, a.configLaggards(p, written))
}

// A wait that expires answers the nodes it has not reached.
func TestWaitConfigPropagatedAnswersWhoLags(t *testing.T) {
	p := naming.Path{Kind: naming.KindSvc, Name: "propagated"}
	a := &DaemonAPI{localhost: "n1"}
	written := time.Now()

	propagationFixture(t, p, []string{"n1", "n2"})
	setConfig(p, "n1", written, "n1", "n2")
	setConfig(p, "n2", written.Add(-time.Minute), "n1", "n2")

	saved := configPropagationSafetyTick
	configPropagationSafetyTick = 10 * time.Millisecond
	t.Cleanup(func() { configPropagationSafetyTick = saved })

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	// No event arrives: the safety tick is what looks again.
	lagging := a.waitConfigPropagated(ctx, &pubsub.Subscription{}, p, written)
	require.Equal(t, []string{"n2"}, lagging)
}
