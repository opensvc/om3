package imon

import (
	"fmt"
	"slices"

	"github.com/opensvc/om3/v3/core/instance"
)

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
		if instance.ConfigData.GetByPathAndNode(t.path, peer) == nil {
			continue
		}
		if m, ok := t.instMonitor[peer]; ok && m.State != instance.MonitorStateInit {
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
