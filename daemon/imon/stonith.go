package imon

import (
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/topology"
)

var (
	nodeStonithAtMap      = make(map[string]time.Time)
	nodeStonithAtMapMutex sync.RWMutex
)

func (t *Manager) setStonith(peerDrop string, peerDropAt time.Time) {
	if peerDrop == "" || peerDropAt.IsZero() {
		return
	}
	if t.instConfig.ActorConfig == nil {
		return
	}
	if !t.instConfig.Stonith {
		return
	}
	if t.instConfig.Topology != topology.Failover {
		return
	}
	if instStatus, ok := t.instStatus[peerDrop]; !ok || instStatus.Avail != status.Up {
		return
	}
	t.log.Tracef("stonith: flag %s, dropped at %s", peerDrop, peerDropAt)
	t.peerDrop = peerDrop
	t.peerDropAt = peerDropAt
}

func (t *Manager) unsetStonith() {
	t.peerDrop = ""
	t.peerDropAt = time.Time{}
}

func (t *Manager) clearStonith(nodename string, avail status.T) {
	if t.peerDrop == "" && t.peerDropAt.IsZero() {
		return
	}
	if avail != status.Up {
		return
	}
	t.log.Tracef("stonith: clear the instance on node %s is started", nodename)
	t.unsetStonith()
}
