package daemonapi

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/daemon/api"
)

// The snapshots are paged newest first, the cursor naming the last one by
// its time, store and id: the runs one sync update writes into two stores
// share their time, and neither is skipped.
func TestPageStoreSnapshots(t *testing.T) {
	t0 := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	items := []api.StoreSnapshot{
		{Store: "a", Id: "r1", CreatedAt: t0},
		{Store: "a", Id: "r2", CreatedAt: t0.Add(time.Hour)},
		{Store: "b", Id: "r2", CreatedAt: t0.Add(time.Hour)},
		{Store: "a", Id: "r3", CreatedAt: t0.Add(2 * time.Hour)},
	}
	limit := 2
	page, err := pageStoreSnapshots(items, &limit, nil, false)
	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	assert.Equal(t, "r3", page.Items[0].Id)
	assert.Equal(t, "a", page.Items[1].Store)
	require.NotNil(t, page.Next)

	page, err = pageStoreSnapshots(items, &limit, page.Next, false)
	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	assert.Equal(t, "b", page.Items[0].Store, "the run sharing the time of the last one is not skipped")
	assert.Equal(t, "r1", page.Items[1].Id)
	assert.Nil(t, page.Next, "the last page")

	bad := "nope"
	_, err = pageStoreSnapshots(items, &limit, &bad, false)
	assert.Error(t, err)
}

// Oldest first, the first snapshot of a period is the first of the page,
// and the cursor goes forward.
func TestPageStoreSnapshotsOldestFirst(t *testing.T) {
	t0 := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	items := []api.StoreSnapshot{
		{Store: "a", Id: "r3", CreatedAt: t0.Add(2 * time.Hour)},
		{Store: "a", Id: "r1", CreatedAt: t0},
		{Store: "a", Id: "r2", CreatedAt: t0.Add(time.Hour)},
	}
	limit := 1
	page, err := pageStoreSnapshots(items, &limit, nil, true)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	assert.Equal(t, "r1", page.Items[0].Id)
	page, err = pageStoreSnapshots(items, &limit, page.Next, true)
	require.NoError(t, err)
	assert.Equal(t, "r2", page.Items[0].Id)
}
