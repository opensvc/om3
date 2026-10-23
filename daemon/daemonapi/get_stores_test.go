package daemonapi

import (
	"testing"

	"github.com/stretchr/testify/assert"

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
