package omcmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/freeze"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/plog"
)

// The paths kept and what they hold stay, the directories leading to them
// stay but their other entries, and the other entries go.
func TestRemoveTree(t *testing.T) {
	dir := t.TempDir()
	for _, p := range []string{
		"backup/a",
		"namespaces/root/svc/foo/status.json",
		"pool/quota/dirquota1/data",
		"pool/other/x",
		"quota-pool.img",
	} {
		p = filepath.Join(dir, p)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0700))
		require.NoError(t, os.WriteFile(p, nil, 0600))
	}
	keep := []string{filepath.Join(dir, "backup"), filepath.Join(dir, "pool/quota")}
	require.NoError(t, removeTree(plog.NewDefaultLogger(), dir, keep))
	assert.FileExists(t, filepath.Join(dir, "backup/a"))
	assert.FileExists(t, filepath.Join(dir, "pool/quota/dirquota1/data"))
	assert.NoDirExists(t, filepath.Join(dir, "pool/other"))
	assert.NoDirExists(t, filepath.Join(dir, "namespaces"))
	assert.NoFileExists(t, filepath.Join(dir, "quota-pool.img"))
}

// TestDropBackup pins that the config backup a join or a leave makes holds
// the config moved aside, and that dropping it removes that backup only, not
// the other entries of the backup directory.
func TestDropBackup(t *testing.T) {
	testhelper.Setup(t)
	t.Cleanup(func() { rawconfig.Load(map[string]string{}) })
	require.NoError(t, os.WriteFile(rawconfig.ClusterConfigFile(), []byte("[cluster]\nsecret = s3cr3t\n"), 0600))
	other := filepath.Join(rawconfig.Paths.Backup, "root.svc.foo_2026-10-29T10:00:00Z.bck")
	require.NoError(t, os.WriteFile(other, []byte("[DEFAULT]\n"), 0600))

	var cmd CmdDaemonCommon
	backup, err := cmd.backupLocalConfig(".pre-cluster-leave-etc")
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(backup, filepath.Base(rawconfig.ClusterConfigFile())))
	assert.NoDirExists(t, rawconfig.Paths.Etc)

	cmd.dropBackup(backup)
	assert.NoDirExists(t, backup)
	assert.FileExists(t, other)
}

// TestDropBackupNone pins that no backup made, the backup directory missing,
// leaves nothing to drop.
func TestDropBackupNone(t *testing.T) {
	testhelper.Setup(t)
	t.Cleanup(func() { rawconfig.Load(map[string]string{}) })
	require.NoError(t, os.RemoveAll(rawconfig.Paths.Backup))

	var cmd CmdDaemonCommon
	backup, err := cmd.backupLocalConfig(".pre-cluster-join-etc")
	require.NoError(t, err)
	assert.Empty(t, backup)
	assert.DirExists(t, rawconfig.Paths.Etc)
	cmd.dropBackup(backup)
	assert.DirExists(t, rawconfig.Paths.Etc)
}

// The node leaves the cluster as the drain left it: frozen, since the time of
// the freeze. A freeze of the cluster it leaves becomes a freeze of the node
// alone, and the rest of the node directory goes.
func TestTheCleanupKeepsTheNodeFrozen(t *testing.T) {
	for _, scope := range []freeze.Scope{freeze.ScopeNode, freeze.ScopeCluster} {
		dir := t.TempDir()
		flag := filepath.Join(dir, "node", "frozen")
		other := filepath.Join(dir, "node", "status.json")
		require.NoError(t, freeze.FreezeScope(flag, scope))
		require.NoError(t, os.WriteFile(other, nil, 0o644))
		at := time.Now().Add(-time.Hour).Truncate(time.Second)
		require.NoError(t, os.Chtimes(flag, at, at))

		log := plog.NewDefaultLogger()
		require.NoError(t, removeTree(log, dir, keepNodeFreeze(log, dir)))

		assert.True(t, freeze.Frozen(flag).Equal(at), "scope %s: frozen at %s", scope, freeze.Frozen(flag))
		assert.Equal(t, freeze.ScopeNode, freeze.ScopeOf(flag, freeze.ScopeCluster, freeze.ScopeNode), scope)
		assert.NoFileExists(t, other)
	}

	// a node not frozen stays so.
	dir := t.TempDir()
	assert.Empty(t, keepNodeFreeze(plog.NewDefaultLogger(), dir))
}
