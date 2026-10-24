package daemonapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/backup"
	"github.com/opensvc/om3/v3/daemon/rbac"
)

func TestMayReadStore(t *testing.T) {
	guest := rbac.NewGrants("guest:ns1")
	assert.True(t, mayReadStores(guest))
	assert.False(t, mayReadStores(rbac.NewGrants()))
	assert.True(t, mayReadStore(guest, []string{"*"}))
	assert.True(t, mayReadStore(guest, []string{"ns2", "ns1"}))
	assert.False(t, mayReadStore(guest, []string{"ns2"}))
	assert.True(t, mayReadStore(rbac.NewGrants("root"), []string{"ns2"}))
	assert.True(t, mayReadStore(rbac.NewGrants("admin"), []string{"ns2"}))
}

func TestMayUseRun(t *testing.T) {
	run := backup.Run{Path: "ns1/svc/web", ClusterID: "c1"}
	admin := rbac.NewGrants("admin:ns1")
	assert.True(t, mayUseRun(admin, "tar1", run, "c1", rbac.RoleAdmin))
	assert.False(t, mayUseRun(rbac.NewGrants("guest:ns1"), "tar1", run, "c1", rbac.RoleAdmin))
	assert.False(t, mayUseRun(rbac.NewGrants("admin:ns2"), "tar1", run, "c1", rbac.RoleAdmin))

	// A namespace is one of this cluster: the run another cluster wrote
	// into a store of the cluster configuration is root's, and the one of
	// a sec store, whose owner shared it, the namespace's.
	assert.False(t, mayUseRun(admin, "tar1", run, "c2", rbac.RoleAdmin))
	assert.True(t, mayUseRun(admin, "from system/sec/stores store#tar2", run, "c2", rbac.RoleAdmin))
	assert.True(t, mayUseRun(rbac.NewGrants("root"), "tar1", run, "c2", rbac.RoleAdmin))

	// The root namespace is the one of a path without one.
	assert.True(t, mayUseRun(rbac.NewGrants("admin:root"), "tar1", backup.Run{Path: "web", ClusterID: "c1"}, "c1", rbac.RoleAdmin))
}

// A namespace sees the stats of its own objects in a store others use too,
// with the space left of the whole store; root and a role on every
// namespace see the whole store.
func TestStoreStatsFor(t *testing.T) {
	free := int64(1000)
	whole := backup.Stats{Runs: 5, Objects: 2, FreeBytes: &free, MissingNodes: []string{"n3"}}
	objects := []backup.ObjectStats{
		{ClusterID: "c1", Path: "ns1/svc/web", Runs: 2, LogicalBytes: 10},
		{ClusterID: "c1", Path: "ns2/svc/db", Runs: 3, LogicalBytes: 20},
	}
	s := storeStatsFor(rbac.NewGrants("guest:ns1"), "tar1", "c1", whole, objects)
	assert.Equal(t, 2, s.Runs)
	assert.Equal(t, 1, s.Objects)
	assert.Equal(t, int64(10), s.LogicalBytes)
	require.NotNil(t, s.FreeBytes)
	assert.Equal(t, free, *s.FreeBytes, "the space left is the store's")
	assert.Equal(t, []string{"n3"}, s.MissingNodes)

	assert.Equal(t, 5, storeStatsFor(rbac.NewGrants("root"), "tar1", "c1", whole, objects).Runs)
	assert.Equal(t, 5, storeStatsFor(rbac.NewGrants("guest"), "tar1", "c1", whole, objects).Runs, "a role on every namespace")
}
