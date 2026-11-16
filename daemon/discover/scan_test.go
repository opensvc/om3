package discover

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/discovered"
	"github.com/opensvc/om3/v3/util/plog"
)

func newScanManager(t *testing.T) *Manager {
	t.Helper()
	discovered.Reset()
	t.Cleanup(discovered.Reset)
	return &Manager{log: plog.NewDefaultLogger(), scanSettleTimeout: time.Second}
}

// The initial discovery settles once every object of the configuration
// files found as the daemon started is discovered: its status is known, or
// its configuration failed to load or is gone (card 308).
func TestInitialScanSettles(t *testing.T) {
	m := newScanManager(t)
	ok := naming.Path{Namespace: "c308", Kind: naming.KindSvc, Name: "ok"}
	bad := naming.Path{Namespace: "c308", Kind: naming.KindSvc, Name: "bad"}
	known := naming.Path{Namespace: "c308", Kind: naming.KindSvc, Name: "known"}
	object.StatusData.Set(known, &object.Status{})
	t.Cleanup(func() { object.StatusData.Unset(known) })

	m.onInitialScan(&initialScan{paths: naming.Paths{ok, bad, known}})
	assert.False(t, discovered.Settled(), "ok and bad are not discovered")
	assert.Len(t, m.scanPending, 2, "the status of known is already there")

	m.scanDiscovered(naming.Path{Namespace: "c308", Kind: naming.KindSvc, Name: "other"})
	assert.False(t, discovered.Settled(), "an object not of the walk")
	m.scanDiscovered(ok)
	assert.False(t, discovered.Settled(), "the status of ok is known, bad is left")
	m.scanDiscovered(bad)
	assert.True(t, discovered.Settled(), "bad failed to load")
}

// No configuration file: settled at once.
func TestInitialScanEmpty(t *testing.T) {
	m := newScanManager(t)
	m.onInitialScan(&initialScan{})
	assert.True(t, discovered.Settled())
}

// An object never discovered does not hold the discovery beyond the timeout.
func TestInitialScanTimeout(t *testing.T) {
	m := newScanManager(t)
	m.onInitialScan(&initialScan{paths: naming.Paths{{Namespace: "c308", Kind: naming.KindSvc, Name: "stuck"}}})
	assert.False(t, discovered.Settled())
	m.onScanSettleTimeout()
	assert.True(t, discovered.Settled())

	// The walk itself not done by the timeout.
	m = newScanManager(t)
	m.onScanSettleTimeout()
	assert.True(t, discovered.Settled())
}
