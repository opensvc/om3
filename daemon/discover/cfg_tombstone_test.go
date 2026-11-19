package discover

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/tombstone"
	"github.com/opensvc/om3/v3/testhelper"
)

// The version of a config a delete or a purge removed, held by a node away
// then, is not fetched from it: installed, it would bring the object back.
func TestAKilledVersionIsNotFetched(t *testing.T) {
	testhelper.Setup(t)
	tombstone.InitData()
	t.Cleanup(tombstone.InitData)
	p, err := naming.ParsePath("test/svc/killed")
	require.NoError(t, err)
	version := time.Date(2026, 11, 17, 15, 33, 47, 0, time.UTC)
	tombstone.Data.Set("n1", tombstone.Record{OrchestrationID: "a", Path: p.String(), Kind: tombstone.KindPurged, Version: version}, true)
	m := newRetryTestManager(t)
	m.fetchConfigFromRemote(fetchArgs{path: p, peer: "n3", updatedAt: version})
	assert.NotContains(t, m.fetcherFrom, p.String(), "the deleted version")
	m.fetchConfigFromRemote(fetchArgs{path: p, peer: "n3", updatedAt: version.Add(-time.Second)})
	assert.NotContains(t, m.fetcherFrom, p.String(), "an older version")
}
