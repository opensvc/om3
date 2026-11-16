package discover

import (
	"slices"
	"strings"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/discovered"
)

type (
	// initialScan lists the objects of the configuration files the fs
	// watcher found as the daemon started.
	initialScan struct {
		paths naming.Paths
	}
)

// onInitialScan starts waiting for the discovery of the objects of the
// configuration files found as the daemon started: the api answers the
// object listings once they are discovered (card 308).
func (t *Manager) onInitialScan(c *initialScan) {
	if discovered.Settled() {
		return
	}
	t.scanPending = make(map[naming.Path]bool, len(c.paths))
	for _, p := range c.paths {
		if object.StatusData.GetByPath(p) != nil {
			continue
		}
		t.scanPending[p] = true
	}
	t.log.Infof("cfg: initial discovery: %d objects found, %d to discover", len(c.paths), len(t.scanPending))
	t.settleScanIfDone()
}

// scanDiscovered marks the object of p discovered: its status is known, or
// its configuration failed to load or is gone.
func (t *Manager) scanDiscovered(p naming.Path) {
	if t.scanPending == nil || !t.scanPending[p] {
		return
	}
	delete(t.scanPending, p)
	t.settleScanIfDone()
}

func (t *Manager) settleScanIfDone() {
	if len(t.scanPending) > 0 || discovered.Settled() {
		return
	}
	t.log.Infof("cfg: initial discovery settled")
	discovered.SetSettled()
}

// onScanSettleTimeout settles the initial discovery an object never shows
// the status of, naming the objects left.
func (t *Manager) onScanSettleTimeout() {
	if discovered.Settled() {
		return
	}
	var l []string
	for p := range t.scanPending {
		l = append(l, p.String())
	}
	slices.Sort(l)
	if t.scanPending == nil {
		t.log.Warnf("cfg: initial discovery settled after %s: the configuration files walk did not end", t.scanSettleTimeout)
	} else {
		t.log.Warnf("cfg: initial discovery settled after %s, %d objects not discovered: %s", t.scanSettleTimeout, len(l), strings.Join(l, ", "))
	}
	discovered.SetSettled()
}
