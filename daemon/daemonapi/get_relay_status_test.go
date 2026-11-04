package daemonapi

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/daemon/api"
)

func TestRelaySinceParsesADuration(t *testing.T) {
	s := "1h"
	d, err := relaySince(api.GetRelayStatusParams{Since: &s})
	require.NoError(t, err)
	assert.Equal(t, time.Hour, d)

	d, err = relaySince(api.GetRelayStatusParams{})
	require.NoError(t, err)
	assert.Zero(t, d, "no since lists every client")

	bad := "soon"
	_, err = relaySince(api.GetRelayStatusParams{Since: &bad})
	assert.Error(t, err)
}

// The relay clients are listed the latest update first, the live ones on top
// of the stale ones, a relay not answering last.
func TestRelayStatusItemsAreSortedLatestUpdateFirst(t *testing.T) {
	now := time.Now()
	items := api.RelayStatusItems{
		{Nodename: "stale", UpdatedAt: now.Add(-22 * time.Hour)},
		{Nodename: "unreachable", Status: "dial tcp: refused"},
		{Nodename: "live", UpdatedAt: now},
	}
	sortRelayStatusItems(items)
	assert.Equal(t, "live", items[0].Nodename)
	assert.Equal(t, "stale", items[1].Nodename)
	assert.Equal(t, "unreachable", items[2].Nodename)
}
