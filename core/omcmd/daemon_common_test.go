package omcmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
