package storeplakar

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/backup"
	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/executable"
	"github.com/opensvc/om3/v3/util/plog"
)

// TestWithTheRealPlakar runs the driver against the plakar binary of the
// host, in the directories of the test, to pin what the fake cannot: the
// tags plakar selects on, the store created by the first write only, the
// space reclaimed after a deletion and the maintenance giving up under the
// lock of a backup. It is skipped on a host without plakar.
func TestWithTheRealPlakar(t *testing.T) {
	bin, err := exec.LookPath("plakar")
	if err != nil {
		t.Skip("no plakar in the PATH")
	}
	e := newTestEnv(t)
	disableSecurityCheck = runDisableSecurityCheck
	getPath = func(string) string { return bin }
	t.Cleanup(func() { getPath = capabilities.GetPath })
	// plakar reads the passphrase from om, which the test does not build.
	om := filepath.Join(t.TempDir(), "om")
	require.NoError(t, os.WriteFile(om, []byte("#!/bin/sh\necho "+passphrase+"\n"), 0755))
	t.Setenv(executable.ExecutableKey, om)
	var logs bytes.Buffer
	e.store.SetLoggerForTest(plog.NewLogger(zerolog.New(&logs).Level(zerolog.InfoLevel)))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	require.NoError(t, os.WriteFile(filepath.Join(e.src, "a"), []byte("content of a\n"), 0644))
	run := run1
	run.ClusterID = "5a0ae2ee-2f5b-4a0c-9d9a-0f4b3a3f4c11"
	other := run1
	other.ID = "r2"
	other.ObjectID = "o2"
	other.Path = "test/svc/app2"
	other.ClusterID = "9e1f2a30-7c44-4c57-8a0b-6d2c5e1b7f22"
	other.ClusterName = "cluster2"

	// No read creates the store, and a store under the head is refused
	// before it is created.
	runs, err := e.store.Runs(ctx, backup.RunFilter{Incomplete: true})
	require.NoError(t, err)
	assert.Empty(t, runs)
	require.Error(t, e.store.Check(ctx, backup.RunFilter{}))
	require.NoError(t, e.store.Delete(ctx, []backup.Run{run}))
	require.Error(t, e.store.Backup(ctx, run, "fs#1", e.root, e.root, nil))
	require.NoDirExists(t, e.storePath())

	// The first write creates it.
	require.NoError(t, e.store.Backup(ctx, run, "fs#1", e.src, e.src, nil))
	require.FileExists(t, filepath.Join(e.storePath(), "CONFIG"))
	// The plakar of the store no longer calls out for security updates.
	require.FileExists(t, filepath.Join(e.dirs.config, securityCheckMarker))
	cookies, err := filepath.Glob(filepath.Join(e.dirs.cache, "cookies", "*", ".disabled-security-check"))
	require.NoError(t, err)
	require.NotEmpty(t, cookies, "plakar records the security check disabled in its cache")
	require.NoError(t, e.store.BackupStream(ctx, run, backup.MemberConfig, strings.NewReader("[DEFAULT]\nid = o1\n")))
	require.NoError(t, e.store.Complete(ctx, run))
	require.NoError(t, e.store.Backup(ctx, other, "fs#1", e.src, e.src, nil))
	require.NoError(t, e.store.Complete(ctx, other))

	// The listing narrowed with the tags of the cluster and of the
	// object holds the runs of that object only: plakar requires both.
	snaps, err := e.store.list(ctx, tagValue{tagCluster, run.ClusterID}, tagValue{tagObject, run.ObjectID})
	require.NoError(t, err)
	require.Len(t, snaps, 3, "the two members and the marker")
	for _, s := range snaps {
		assert.Equal(t, run.ID, s.Tags[tagRun])
	}
	snaps, err = e.store.list(ctx, tagValue{tagCluster, run.ClusterID}, tagValue{tagObject, other.ObjectID})
	require.NoError(t, err)
	assert.Empty(t, snaps, "the tags of a -tag are all required")
	require.NoError(t, e.store.Close())

	found, err := backup.FindRun(ctx, e.store, backup.RunFilter{ObjectID: run.ObjectID, Cluster: run.ClusterID})
	require.NoError(t, err)
	assert.Equal(t, run.ID, found.ID)
	assert.False(t, found.CompletedAt.IsZero())
	assert.Equal(t, []string{"fs#1", backup.MemberConfig}, found.MemberNames())
	runs, err = e.store.Runs(ctx, backup.RunFilter{})
	require.NoError(t, err)
	require.Len(t, runs, 2)
	stats, err := e.store.ObjectStats(ctx)
	require.NoError(t, err)
	assert.Len(t, stats, 2)

	// The members are read back from the snapshots the listing names.
	require.NoError(t, e.store.Restore(ctx, found, "fs#1", e.dst))
	b, err := os.ReadFile(filepath.Join(e.dst, "a"))
	require.NoError(t, err)
	assert.Equal(t, "content of a\n", string(b))
	b, err = backup.ReadMember(ctx, e.store, found, backup.MemberConfig)
	require.NoError(t, err)
	assert.Equal(t, "[DEFAULT]\nid = o1\n", string(b))
	require.NoError(t, e.store.Check(ctx, backup.RunFilter{ID: run.ID}))
	require.NoError(t, e.store.Check(ctx, backup.RunFilter{}))

	// A deletion removes the run and runs the maintenance.
	logs.Reset()
	require.NoError(t, e.store.Delete(ctx, []backup.Run{run}))
	assert.Contains(t, logs.String(), "at @store maintenance")
	assert.NotContains(t, logs.String(), "maintenance skipped")
	runs, err = e.store.Runs(ctx, backup.RunFilter{})
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, other.ID, runs[0].ID)

	// A deletion removing nothing runs none.
	logs.Reset()
	require.NoError(t, e.store.Delete(ctx, []backup.Run{run}))
	assert.NotContains(t, logs.String(), "at @store maintenance")

	// Under the lock of a backup, here one whose stream stays open, the
	// maintenance gives up and the driver says it skipped it.
	pr, pw := io.Pipe()
	done := make(chan error, 1)
	writer := e.store
	third := run
	third.ID = "r3"
	go func() {
		done <- writer.BackupStream(ctx, third, backup.MemberConfig, pr)
	}()
	logs.Reset()
	skipped := false
	for i := 0; i < 20 && !skipped; i++ {
		time.Sleep(500 * time.Millisecond)
		require.NoError(t, e.store.maintain(ctx))
		skipped = strings.Contains(logs.String(), "maintenance skipped: repository busy")
	}
	_, err = pw.Write([]byte("x"))
	require.NoError(t, err)
	require.NoError(t, pw.Close())
	require.NoError(t, <-done)
	assert.True(t, skipped, "the maintenance under the lock of a backup is skipped, not failed")
}

// sftpHostEnv names a host the user running the tests reaches by ssh, with
// no password, for TestWithTheRealPlakarOverSFTP.
const sftpHostEnv = "OM_TEST_PLAKAR_SFTP_HOST"

// TestWithTheRealPlakarOverSFTP runs a backup and a restore through the sftp
// plugin an operator installed, in the data directory plakar installs the
// plugins in: the stores share it, so the backends installed are theirs. It
// needs plakar with its sftp plugin, and the name of a host in
// OM_TEST_PLAKAR_SFTP_HOST, which may be the host running the test: the
// store is a temporary directory of this host, reached there.
func TestWithTheRealPlakarOverSFTP(t *testing.T) {
	host := os.Getenv(sftpHostEnv)
	if host == "" {
		t.Skip("no " + sftpHostEnv)
	}
	bin, err := exec.LookPath("plakar")
	if err != nil {
		t.Skip("no plakar in the PATH")
	}
	e := newTestEnv(t)
	dataDir = defaultDataDir
	disableSecurityCheck = runDisableSecurityCheck
	getPath = func(string) string { return bin }
	t.Cleanup(func() { getPath = capabilities.GetPath })
	om := filepath.Join(t.TempDir(), "om")
	require.NoError(t, os.WriteFile(om, []byte("#!/bin/sh\necho "+passphrase+"\n"), 0755))
	t.Setenv(executable.ExecutableKey, om)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	dir := t.TempDir()
	e.store = newStore(t, "[store#lab]\ntype = plakar\nurl = sftp://"+host+dir+"\npassphrase = from system/sec/backup key passphrase\n")
	assert.False(t, e.store.IsLocal())
	// A remote store is created by hand, as the operator does.
	_, err = e.store.prepare()
	require.NoError(t, err)
	_, err = e.store.run(ctx, zerolog.InfoLevel, "create")
	require.NoError(t, err, "the sftp backend of the plugins installed is found")

	require.NoError(t, os.WriteFile(filepath.Join(e.src, "a"), []byte("content of a\n"), 0644))
	require.NoError(t, e.store.Backup(ctx, run1, "fs#1", e.src, e.src, nil))
	require.NoError(t, e.store.Complete(ctx, run1))
	runs, err := e.store.Runs(ctx, backup.RunFilter{})
	require.NoError(t, err)
	require.Len(t, runs, 1)
	require.NoError(t, e.store.Restore(ctx, runs[0], "fs#1", e.dst))
	b, err := os.ReadFile(filepath.Join(e.dst, "a"))
	require.NoError(t, err)
	assert.Equal(t, "content of a\n", string(b))
}
