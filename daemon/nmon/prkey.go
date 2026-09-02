package nmon

import (
	"sort"
	"strings"

	"github.com/opensvc/om3/v3/core/node"
)

// checkPRKey warns when a peer announces the scsi3 persistent
// reservation key of this node.
//
// The key identifies this node to the storage: two nodes holding the
// same one can each preempt the reservations of the other, which is the
// registration a scsireserv resource relies on to keep a peer away from
// a device it has not been given. A duplicate is nearly always a
// node.conf copied to a peer without redacting the prkey, and nothing
// tells the operator until a takeover does the wrong thing.
//
// The check is on the configuration, not on the storage: it sees the
// nodes of this cluster, and not the other clusters registering their
// own keys on the same luns.
func (t *Manager) checkPRKey() {
	peers := t.duplicatePRKeyPeers()
	if len(peers) == 0 {
		if len(t.prKeyPeers) > 0 {
			t.log.Infof("node prkey %s is not used by a peer anymore", t.nodeConfig.PRKey)
			t.prKeyPeers = nil
		}
		return
	}
	if equalStrings(peers, t.prKeyPeers) {
		// Already said, and it is said again on every config change of
		// every node until it is fixed.
		return
	}
	t.prKeyPeers = peers
	t.log.
		Attr("prkey", t.nodeConfig.PRKey).
		Attr("peers", peers).
		Warnf("node prkey %s is also the prkey of %s: a scsi3 reservation of one is preemptable by the other, set a different node.prkey on all but one",
			t.nodeConfig.PRKey, strings.Join(peers, ", "))
}

// duplicatePRKeyPeers returns the peers announcing the prkey of this
// node, sorted.
func (t *Manager) duplicatePRKeyPeers() []string {
	prKey := t.nodeConfig.PRKey
	if prKey == "" {
		return nil
	}
	peers := make([]string, 0)
	for _, e := range node.ConfigData.GetAll() {
		if e.Node == t.localhost {
			continue
		}
		if e.Value == nil {
			continue
		}
		if e.Value.PRKey == prKey {
			peers = append(peers, e.Node)
		}
	}
	if len(peers) == 0 {
		return nil
	}
	sort.Strings(peers)
	return peers
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
