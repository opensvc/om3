package commoncmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/daemon/api"
)

// The times of a listing are shown in the zone of the requester, whatever
// the offset the store or the node answering wrote them with.
func TestLocalSnapshotTimes(t *testing.T) {
	saved := time.Local
	t.Cleanup(func() { time.Local = saved })
	time.Local = time.FixedZone("JST", 9*3600)

	utc := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	cest := time.Date(2026, 10, 22, 14, 0, 0, 0, time.FixedZone("CEST", 2*3600))
	store := []api.StoreSnapshot{{CreatedAt: utc, CompletedAt: &cest}, {CreatedAt: cest}}
	LocalStoreSnapshots(store)
	assert.Equal(t, "2026-10-23T21:00:00+09:00", store[0].CreatedAt.Format(time.RFC3339))
	assert.Equal(t, "2026-10-22T21:00:00+09:00", store[0].CompletedAt.Format(time.RFC3339))
	assert.Nil(t, store[1].CompletedAt)

	sync := api.SyncSnapshotItems{{Data: api.SyncSnapshot{CreatedAt: utc}}}
	LocalSyncSnapshots(sync)
	assert.Equal(t, "2026-10-23T21:00:00+09:00", sync[0].Data.CreatedAt.Format(time.RFC3339))
}
