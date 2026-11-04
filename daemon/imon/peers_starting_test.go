package imon

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/util/plog"
)

// A purge or a delete waits for an alive peer whose instance config is
// published and whose instance monitor starts, as on a node booting: gone
// on without it, the peer is never told, and keeps its instance. A peer
// whose instance is already deleted, with no config, or a peer not alive,
// is not waited for.
func TestAPurgeWaitsForThePeersWhoseMonitorStarts(t *testing.T) {
	p := naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "s244"}
	for _, n := range []string{"n2", "n3", "n4"} {
		instance.ConfigData.Set(p, n, &instance.Config{})
		defer instance.ConfigData.Unset(p, n)
	}
	m := &Manager{
		localhost:  "n1",
		path:       p,
		scopeNodes: []string{"n1", "n2", "n3", "n4", "n5"},
		nodeStatus: map[string]node.Status{"n2": {}, "n3": {}, "n5": {}},
		instMonitor: map[string]instance.Monitor{
			"n3": {State: instance.MonitorStateIdle},
		},
		log: plog.NewDefaultLogger(),
	}
	// n2 alive, config, no monitor: starting. n3: monitor up. n4: not
	// alive. n5: alive, no config: deleted already.
	assert.Equal(t, []string{"n2"}, m.peersStarting())
	assert.True(t, m.waitPeersStarting())

	m.instMonitor["n2"] = instance.Monitor{State: instance.MonitorStateInit}
	assert.Equal(t, []string{"n2"}, m.peersStarting(), "a monitor in init has not taken the orchestration on")

	m.instMonitor["n2"] = instance.Monitor{State: instance.MonitorStateIdle}
	assert.Empty(t, m.peersStarting())
	assert.False(t, m.waitPeersStarting())
}
