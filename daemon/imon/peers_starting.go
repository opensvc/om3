package imon

import (
	"fmt"
	"slices"
	"time"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

// servedStartingTimeout bounds the time a peer this node served the config
// to counts as starting, its instance monitor not seen yet.
var servedStartingTimeout = 10 * time.Second

// peersStarting returns the alive nodes of the scope whose instance config
// is published, but not their instance monitor yet, or one still in init:
// their monitor starts, as on a node booting, and will take the global
// expect on. A node with no instance config, as one whose instance a purge
// already deleted, is not waited for.
func (t *Manager) peersStarting() []string {
	var l []string
	for _, peer := range t.scopeNodes {
		if peer == t.localhost {
			continue
		}
		if _, ok := t.nodeStatus[peer]; !ok {
			// a node not alive is not waited for.
			continue
		}
		if m, ok := t.instMonitor[peer]; ok && m.State != instance.MonitorStateInit {
			// Started: its monitor gone later is its instance
			// deleted, not starting.
			delete(t.servedAt, peer)
			continue
		}
		if instance.ConfigData.GetByPathAndNode(t.path, peer) == nil && !t.servedRecently(peer) {
			continue
		}
		l = append(l, peer)
	}
	slices.Sort(l)
	return l
}

// waitPeersStarting says the purge or the delete waits for the instance
// monitors of peers starting: gone on without them, the instance of the
// node first done is deleted with its monitor, which held the global
// expect, and a peer starting after, never told, keeps its instance, whose
// config comes back on the others.
func (t *Manager) waitPeersStarting() bool {
	l := t.peersStarting()
	if len(l) == 0 {
		t.waitPeersStartingMsg = ""
		return false
	}
	if msg := fmt.Sprintf("wait for the instance monitor of %v, starting", l); msg != t.waitPeersStartingMsg {
		t.log.Infof("%s", msg)
		t.waitPeersStartingMsg = msg
	}
	return true
}

// onInstanceConfigServed records the peer this node served the config to:
// it installs it, its instance coming, before its config, published by
// heartbeat, reaches this node. A delete or a purge went on without it,
// the leader first, and was said over with its instance left (qatests 308).
func (t *Manager) onInstanceConfigServed(c *msgbus.InstanceConfigServed) {
	if t.servedAt == nil {
		t.servedAt = make(map[string]time.Time)
	}
	t.servedAt[c.Node] = time.Now()
	t.onChange()
	// A peer whose monitor never shows is no longer waited for once its
	// mark expires: the orchestration is looked at again then.
	t.armOrchestrationClearTimer(time.Now())
}

// servedRecently says this node served the config to the peer less than
// servedStartingTimeout ago, of the local clock.
func (t *Manager) servedRecently(peer string) bool {
	at, ok := t.servedAt[peer]
	if !ok {
		return false
	}
	if time.Since(at) >= servedStartingTimeout {
		delete(t.servedAt, peer)
		return false
	}
	return true
}
