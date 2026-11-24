package storeplakar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/opensvc/om3/v3/core/backup"
	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/executable"
	"github.com/opensvc/om3/v3/util/plog"
)

const passphrase = "s3cret-passphrase-not-to-be-seen"

// fakePlakar records each call (argv as a json array, the environment) in
// the calls file, copies its stdin to stdin.N, replays reply.N (or reply) on
// stdout and stderr.N (or stderr) on stderr, and exits with status.N (or
// status, or 0), N being the call number.
const fakePlakar = `#!/bin/sh
dir=$(dirname "$0")
touch "$dir/calls"
n=$(($(wc -l < "$dir/calls") + 1))
{
	printf '['
	first=1
	for a in "$@"; do
		[ $first = 1 ] || printf ','
		first=0
		printf '"%s"' "$(printf '%s' "$a" | sed 's/\\/\\\\/g; s/"/\\"/g')"
	done
	printf ']\n'
} >>"$dir/calls"
env >"$dir/env.$n"
cat >"$dir/stdin.$n"
if [ -f "$dir/reply.$n" ]; then cat "$dir/reply.$n"; elif [ -f "$dir/reply" ]; then cat "$dir/reply"; fi
if [ -f "$dir/stderr.$n" ]; then cat "$dir/stderr.$n" >&2; elif [ -f "$dir/stderr" ]; then cat "$dir/stderr" >&2; fi
s=0
if [ -f "$dir/status.$n" ]; then s=$(cat "$dir/status.$n"); elif [ -f "$dir/status" ]; then s=$(cat "$dir/status"); fi
exit $s
`

var fakeDir string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "plakar.")
	if err != nil {
		panic(err)
	}
	fakeDir = dir
	if err := os.WriteFile(filepath.Join(dir, "plakar"), []byte(fakePlakar), 0755); err != nil {
		panic(err)
	}
	caps, _ := json.Marshal([]string{
		capabilities.MakePath("plakar", filepath.Join(dir, "plakar")),
		drvID.Cap(),
	})
	if err := os.WriteFile(filepath.Join(dir, "capabilities.json"), caps, 0644); err != nil {
		panic(err)
	}
	capabilities.SetCacheFile(filepath.Join(dir, "capabilities.json"))
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// resetFake forgets the calls made and the replies set.
func resetFake(t *testing.T) {
	t.Helper()
	entries, err := os.ReadDir(fakeDir)
	require.NoError(t, err)
	for _, e := range entries {
		switch e.Name() {
		case "plakar", "capabilities.json":
			continue
		}
		require.NoError(t, os.Remove(filepath.Join(fakeDir, e.Name())))
	}
}

func setFake(t *testing.T, name string, content []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(fakeDir, name), content, 0644))
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "run", name))
	require.NoError(t, err)
	return b
}

// calls returns the argv of every call made, one per line.
func calls(t *testing.T) [][]string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(fakeDir, "calls"))
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	l := make([][]string, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		var argv []string
		require.NoError(t, json.Unmarshal([]byte(line), &argv), line)
		l = append(l, argv)
	}
	return l
}

// testEnv is a node holding the sec the passphrase keyword names and one
// [store#lab] section of type plakar on a local store directory.
type testEnv struct {
	store *T
	dirs  dirs
	root  string
	src   string
	dst   string
}

func (e testEnv) storePath() string {
	return filepath.Join(e.root, "store")
}

func newStore(t *testing.T, config string) *T {
	t.Helper()
	cfg, err := xconfig.NewObject("", []byte(config))
	require.NoError(t, err)
	drv := backup.GetDriver("plakar")
	require.NotNil(t, drv, "the plakar driver registers itself")
	drv.SetName("lab")
	drv.SetConfig(cfg)
	return drv.(*T)
}

func newTestEnv(t *testing.T) testEnv {
	t.Helper()
	resetFake(t)
	// The fake records every plakar call: the security check, disabled once
	// per store, is the integration test's to run.
	disableSecurityCheck = func(dirs) error { return nil }
	t.Cleanup(func() { disableSecurityCheck = runDisableSecurityCheck })
	// So is the call readying the cache, the first of each driver.
	warmUp = func(*T, dirs) error { return nil }
	t.Cleanup(func() { warmUp = runWarmUp })
	env := testhelper.Setup(t)
	dataDir = func() string { return filepath.Join(env.Root, "plakar-data") }
	t.Cleanup(func() { dataDir = defaultDataDir })
	// Paths.Run is /run/opensvc whatever the root: keep the test in its root.
	rawconfig.Paths.Run = filepath.Join(env.Root, "run")
	// rawconfig.Load points the capabilities at the root: back to the fake.
	capabilities.SetCacheFile(filepath.Join(fakeDir, "capabilities.json"))
	// A sec is encrypted with the cluster secret.
	cfg := &cluster.Config{Name: "cluster1"}
	cfg.SetSecret("070fd9169fc111ec9c5017409407c6ab")
	cluster.ConfigData.Set(cfg)
	p, err := naming.ParsePath("system/sec/backup")
	require.NoError(t, err)
	sec, err := object.NewSec(p)
	require.NoError(t, err)
	require.NoError(t, sec.AddKey("passphrase", []byte(passphrase)))

	t.Setenv(executable.ExecutableKey, "/usr/bin/om")

	e := testEnv{root: env.Root, src: filepath.Join(env.Root, "src"), dst: filepath.Join(env.Root, "dst")}
	require.NoError(t, os.MkdirAll(e.src, 0755))
	e.store = newStore(t, fmt.Sprintf("[store#lab]\ntype = plakar\nurl = %s\npassphrase = from system/sec/backup key passphrase\n", e.storePath()))
	e.dirs, err = e.store.dirs()
	require.NoError(t, err)
	return e
}

// createStore makes the local store look created, so no create call runs.
func (e testEnv) createStore(t *testing.T) {
	t.Helper()
	require.NoError(t, os.MkdirAll(e.storePath(), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(e.storePath(), "CONFIG"), []byte("x"), 0600))
}

// argv is the command line plakar receives in the environment: the private
// directories, then the store and the verb.
func (e testEnv) argv(at string, args ...string) []string {
	return append([]string{"-configdir", e.dirs.config, "-cachedir", e.dirs.cache, "-datadir", e.dirs.data, "at", at}, args...)
}

var (
	t0 = time.Date(2026, 10, 27, 10, 0, 0, 123000000, time.UTC)

	run1 = backup.Run{ID: "r1", ClusterID: "c1", ClusterName: "cluster1", Path: "test/svc/app1", ObjectID: "o1", Node: "n1", CreatedAt: t0}

	run1Tags = "om3.run=r1,om3.object=o1,om3.path=test%2Fsvc%2Fapp1,om3.cluster=c1,om3.cluster_name=cluster1,om3.node=n1,om3.created=2026-10-27T10%3A00%3A00.123Z"
)

// lsLine is a line of plakar ls -uuid -tags for a snapshot of the run,
// tagged as backupArgs tags it, plus tags.
func lsLine(ts time.Time, id string, run backup.Run, tags ...string) string {
	return fmt.Sprintf("%s %s 14 B 0s /tmp/data tags=%s\n", ts.UTC().Format(time.RFC3339), id, backupArgs(run, tags...)[1])
}

// TestBackupTagsTheMemberAndNamesItAfterThePath pins the backup command
// line: the tags of the run and of the member, escaped, and the name.
func TestBackupTagsTheMemberAndNamesItAfterThePath(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	run := run1
	run.Protected = true
	run.Note = "before the upgrade"
	require.NoError(t, e.store.Backup(context.Background(), run, "fs#1", e.src, "/srv/app1", nil))
	assert.Equal(t, [][]string{e.argv("@store", "backup",
		"-tag", run1Tags+",om3.protected,om3.note=before+the+upgrade,om3.member=fs%231,om3.kind=dir,om3.head=%2Fsrv%2Fapp1",
		"-name", "test/svc/app1",
		e.src,
	)}, calls(t))
}

// TestBackupAnchorsTheExcludesAtTheSource pins the -ignore patterns: plakar
// anchors a pattern holding a slash at the root of the filesystem, so an
// anchored exclude is prefixed with the source.
func TestBackupAnchorsTheExcludesAtTheSource(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	excludes := []string{"/tmp", "*.log", "a/b", backup.LiteralPattern("sub/st*r"), "!keep", ""}
	require.NoError(t, e.store.Backup(context.Background(), run1, "fs#1", "/srv/da*ta", "/srv/data", excludes))
	l := calls(t)
	require.Len(t, l, 1)
	assert.Equal(t, []string{
		"-ignore", `/srv/da\*ta/tmp`,
		"-ignore", "*.log",
		"-ignore", `/srv/da\*ta/a/b`,
		"-ignore", `/srv/da\*ta/sub/st\*r`,
		"-ignore", `\!keep`,
		"/srv/da*ta",
	}, l[0][len(l[0])-11:])
}

// TestBackupRefusesAStoreUnderTheSource pins the refusal of a local store
// inside the directory it would back up, before any call: the store, not
// created yet, is not created in the head to be refused next.
func TestBackupRefusesAStoreUnderTheSource(t *testing.T) {
	e := newTestEnv(t)
	err := e.store.Backup(context.Background(), run1, "fs#1", e.root, e.root, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "a store is never inside a head it backs up")
	assert.Empty(t, calls(t), "no create, no backup")
	assert.NoDirExists(t, e.storePath())
}

// TestBackupRefusesAUrlItCannotJudge pins the url error reaching the caller
// of a backup, before any call: a url not read is not a store outside the
// head.
func TestBackupRefusesAUrlItCannotJudge(t *testing.T) {
	e := newTestEnv(t)
	s := newStore(t, "[store#lab]\ntype = plakar\nurl = fs://srv/plakar\npassphrase = from system/sec/backup key passphrase\n")
	err := s.Backup(context.Background(), run1, "fs#1", e.src, e.src, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `url "fs://srv/plakar" is not an absolute path`)
	assert.Empty(t, calls(t))
}

// TestBackupStreamSendsTheBytesOnStdin pins the stdin:// source and the
// bytes plakar reads, never written to a file.
func TestBackupStreamSendsTheBytesOnStdin(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	setFake(t, "reply", fixture(t, "30-stdio-backup.txt"))
	require.NoError(t, e.store.BackupStream(context.Background(), run1, "refs.json", strings.NewReader(`{"sec/db":{"password":"c2VjcmV0"}}`)))
	assert.Equal(t, [][]string{e.argv("@store", "backup", "-tag", run1Tags+",om3.member=refs.json,om3.kind=stream", "-name", "test/svc/app1", "stdin:///refs.json")}, calls(t))
	stdin, err := os.ReadFile(filepath.Join(fakeDir, "stdin.1"))
	require.NoError(t, err)
	assert.Equal(t, `{"sec/db":{"password":"c2VjcmV0"}}`, string(stdin), "the bytes reach plakar on stdin")
}

// TestCompleteWritesAnEmptyMarker pins the marker of a completed run: an
// empty stream tagged as the run, complete and when.
func TestCompleteWritesAnEmptyMarker(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	require.NoError(t, e.store.Complete(context.Background(), run1))
	l := calls(t)
	require.Len(t, l, 1)
	tags := l[0][len(e.argv("@store", "backup", "-tag"))]
	assert.True(t, strings.HasPrefix(tags, run1Tags+",om3.complete,om3.completed=2"), tags)
	assert.Equal(t, []string{"-name", "test/svc/app1", "stdin:///complete"}, l[0][len(l[0])-3:])
	stdin, err := os.ReadFile(filepath.Join(fakeDir, "stdin.1"))
	require.NoError(t, err)
	assert.Empty(t, stdin)
}

// TestRunsTellsAnIncompleteRunFromACompletedOne pins a run listed once its
// marker is written, and an incomplete run listed only when asked, with the
// members in the order they were saved, and a snapshot of no run left out.
func TestRunsTellsAnIncompleteRunFromACompletedOne(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	run2 := run1
	run2.ID = "r2"
	run2.CreatedAt = t0.Add(time.Hour)
	// plakar lists the newest first.
	setFake(t, "reply", []byte(
		lsLine(t0.Add(time.Hour), "s4", run2, "om3.member=fs%231,om3.kind=dir,om3.head=%2Fsrv%2Fapp1")+
			lsLine(t0.Add(2*time.Minute), "s3", run1, "om3.complete,om3.completed=2026-10-27T10%3A02%3A00Z")+
			lsLine(t0.Add(time.Minute), "s2", run1, "om3.member=config,om3.kind=stream")+
			lsLine(t0, "s1", run1, "om3.member=fs%231,om3.kind=dir,om3.head=%2Fsrv%2Fapp1")+
			string(fixture(t, "08-ls-tags.txt"))))
	runs, err := e.store.Runs(context.Background(), backup.RunFilter{})
	require.NoError(t, err)
	require.Len(t, runs, 1)
	want := run1
	want.CompletedAt = time.Date(2026, 10, 27, 10, 2, 0, 0, time.UTC)
	want.Members = []backup.Member{
		{Name: "fs#1", Kind: backup.MemberDir, Head: "/srv/app1", LogicalBytes: 14},
		{Name: "config", Kind: backup.MemberStream, LogicalBytes: 14},
	}
	want.LogicalBytes = 28
	assert.Equal(t, want, runs[0])
	assert.Equal(t, [][]string{e.argv("@store", "ls", "-uuid", "-tags")}, calls(t))

	runs, err = e.store.Runs(context.Background(), backup.RunFilter{Incomplete: true})
	require.NoError(t, err)
	require.Len(t, runs, 2)
	assert.Equal(t, "r1", runs[0].ID, "oldest first")
	assert.Equal(t, "r2", runs[1].ID)
	assert.True(t, runs[1].CompletedAt.IsZero())
}

// TestRunsFollowTheObjectAndTheClusterRenamed pins the path an object is
// listed by, the one of its latest marker, and the cluster named as its
// latest marker says.
func TestRunsFollowTheObjectAndTheClusterRenamed(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	run2 := run1
	run2.ID = "r2"
	run2.CreatedAt = t0.Add(time.Hour)
	run2.Path = "test/svc/app2"
	run2.ClusterName = "cluster2"
	setFake(t, "reply", []byte(
		lsLine(t0.Add(time.Hour), "s2", run2, "om3.complete,om3.completed=2026-10-27T11%3A00%3A00Z")+
			lsLine(t0, "s1", run1, "om3.complete,om3.completed=2026-10-27T10%3A00%3A00Z")))
	runs, err := e.store.Runs(context.Background(), backup.RunFilter{Path: "test/svc/app2"})
	require.NoError(t, err)
	require.Len(t, runs, 2, "the runs taken before the rename are found by the new path")
	assert.Equal(t, "test/svc/app1", runs[0].Path, "a run says the path the object had then")
	assert.Equal(t, "cluster2", runs[0].ClusterName, "the cluster named as it is now")

	runs, err = e.store.Runs(context.Background(), backup.RunFilter{Path: "test/svc/app1"})
	require.NoError(t, err)
	assert.Empty(t, runs, "the former path names no object")
}

// TestRunsKeepTheProtectionAndTheNote pins a protected run and a note
// holding spaces and an equal sign read back as written.
func TestRunsKeepTheProtectionAndTheNote(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	run := run1
	run.Protected = true
	run.Note = "before the upgrade = 2.4"
	setFake(t, "reply", []byte(lsLine(t0, "s1", run, "om3.complete,om3.completed=2026-10-27T10%3A00%3A00Z")))
	runs, err := e.store.Runs(context.Background(), backup.RunFilter{})
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.True(t, runs[0].Protected)
	assert.Equal(t, "before the upgrade = 2.4", runs[0].Note)
}

// TestRestoreFindsTheSnapshotOfTheMember pins the lookup of the member
// snapshot then its restore under dst, and ErrNotFound for a member the
// store does not hold.
func TestRestoreFindsTheSnapshotOfTheMember(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	setFake(t, "reply.1", []byte(lsLine(t0, "s1", run1, "om3.member=fs%231,om3.kind=dir")))
	require.NoError(t, e.store.Restore(context.Background(), run1, "fs#1", e.dst))
	assert.Equal(t, [][]string{
		e.argv("@store", "ls", "-uuid", "-tags", "-tag", "om3.run=r1"),
		e.argv("@store", "restore", "-to", e.dst, "s1"),
	}, calls(t))
	assert.DirExists(t, e.dst)

	resetFake(t)
	err := e.store.Restore(context.Background(), run1, "fs#2", e.dst)
	assert.ErrorIs(t, err, backup.ErrNotFound)
	assert.Empty(t, calls(t), "no restore of a member not found, told from the listing of the run already read")
}

// TestCatReadsAStreamMemberOnly pins the cat of the file of a stream
// member, and the refusal of a member saved from a directory.
func TestCatReadsAStreamMemberOnly(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	setFake(t, "reply.1", []byte(lsLine(t0, "s2", run1, "om3.member=config,om3.kind=stream")))
	setFake(t, "reply.2", fixture(t, "33-cat-refs.txt"))
	run := run1
	run.Members = []backup.Member{{Name: "config", Kind: backup.MemberStream}, {Name: "fs#1", Kind: backup.MemberDir}}
	rc, err := e.store.Cat(context.Background(), run, "config")
	require.NoError(t, err)
	b, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, string(fixture(t, "33-cat-refs.txt")), string(b))
	assert.Equal(t, e.argv("@store", "cat", "s2:/config"), calls(t)[1])

	_, err = e.store.Cat(context.Background(), run, "fs#1")
	assert.Error(t, err)
}

// TestDeleteRemovesEverySnapshotOfTheRuns pins one rm of all the snapshots
// of each run, its marker too, a run with no snapshot removing nothing, then
// one maintenance reclaiming the space of what was removed.
func TestDeleteRemovesEverySnapshotOfTheRuns(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	setFake(t, "reply.1", fixture(t, "120-rm-apply-removed.txt"))
	setFake(t, "reply.2", fixture(t, "92-rm-apply-nomatch.txt"))
	setFake(t, "reply.3", fixture(t, "121-maintenance.txt"))
	run2 := run1
	run2.ID = "r2"
	require.NoError(t, e.store.Delete(context.Background(), []backup.Run{run1, run2}))
	assert.Equal(t, [][]string{
		e.argv("@store", "rm", "-apply", "-tag", "om3.run=r1"),
		e.argv("@store", "rm", "-apply", "-tag", "om3.run=r2"),
		e.argv("@store", "maintenance"),
	}, calls(t))
}

// TestDeleteOfNoSnapshotRunsNoMaintenance pins no maintenance, which wants
// the store for itself, after a deletion removing nothing, as the one of a
// run failed before its first member.
func TestDeleteOfNoSnapshotRunsNoMaintenance(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	setFake(t, "reply", fixture(t, "92-rm-apply-nomatch.txt"))
	require.NoError(t, e.store.Delete(context.Background(), []backup.Run{run1}))
	assert.Equal(t, [][]string{e.argv("@store", "rm", "-apply", "-tag", "om3.run=r1")}, calls(t))
}

// TestMaintenanceUnderALockIsSkipped pins the maintenance giving up under
// the lock of another plakar as no error of the deletion, whether plakar
// exits 0 or not, and said in the log.
func TestMaintenanceUnderALockIsSkipped(t *testing.T) {
	for _, status := range []string{"0", "1"} {
		e := newTestEnv(t)
		e.createStore(t)
		var buf bytes.Buffer
		e.store.SetLoggerForTest(plog.NewLogger(zerolog.New(&buf).Level(zerolog.InfoLevel)))
		setFake(t, "reply.1", fixture(t, "120-rm-apply-removed.txt"))
		setFake(t, "reply.2", fixture(t, "122-maintenance-locked.txt"))
		setFake(t, "status.2", []byte(status))
		require.NoError(t, e.store.Delete(context.Background(), []backup.Run{run1}), "exit %s", status)
		assert.Len(t, calls(t), 2, "exit %s", status)
		assert.Contains(t, buf.String(), "maintenance skipped: repository busy", "exit %s", status)
	}
}

// TestMaintenanceFailureIsReported pins the error of a maintenance failing
// for another reason than a lock: the runs are deleted, their space is not
// reclaimed, and the store would grow unseen.
func TestMaintenanceFailureIsReported(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	setFake(t, "reply.1", fixture(t, "120-rm-apply-removed.txt"))
	setFake(t, "status.2", []byte("1"))
	setFake(t, "stderr.2", []byte("plakar: no space left on device\n"))
	err := e.store.Delete(context.Background(), []backup.Run{run1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "the runs are deleted, their space is not reclaimed: plakar maintenance: ")
	assert.Contains(t, err.Error(), "no space left on device")
}

// TestCheckVerifiesTheStoreOrTheSnapshotsOfTheRuns pins the whole store
// check for a zero filter, and the check of the snapshots of the runs a
// filter selects.
func TestCheckVerifiesTheStoreOrTheSnapshotsOfTheRuns(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	require.NoError(t, e.store.Check(context.Background(), backup.RunFilter{}))
	assert.Equal(t, [][]string{e.argv("@store", "check")}, calls(t))

	resetFake(t)
	run2 := run1
	run2.ID = "r2"
	setFake(t, "reply.1", []byte(
		lsLine(t0.Add(time.Hour), "s3", run2, "om3.complete,om3.completed=2026-10-27T11%3A00%3A00Z")+
			lsLine(t0, "s2", run1, "om3.complete,om3.completed=2026-10-27T10%3A00%3A00Z")+
			lsLine(t0, "s1", run1, "om3.member=fs%231,om3.kind=dir")))
	require.NoError(t, e.store.Check(context.Background(), backup.RunFilter{ID: "r1"}))
	assert.Equal(t, e.argv("@store", "check", "s1", "s2"), calls(t)[1])
}

// TestObjectStatsCountTheRunsOfEachObject pins one record per object, the
// incomplete runs counted.
func TestObjectStatsCountTheRunsOfEachObject(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	run2 := run1
	run2.ID = "r2"
	run3 := run1
	run3.ID = "r3"
	run3.ObjectID = "o2"
	run3.Path = "test/svc/app2"
	setFake(t, "reply", []byte(
		lsLine(t0, "s3", run3, "om3.member=fs%231,om3.kind=dir")+
			lsLine(t0, "s2", run2, "om3.member=fs%231,om3.kind=dir")+
			lsLine(t0, "s1", run1, "om3.complete,om3.completed=2026-10-27T10%3A00%3A00Z")))
	stats, err := e.store.ObjectStats(context.Background())
	require.NoError(t, err)
	require.Len(t, stats, 2)
	assert.Equal(t, "o1", stats[0].ObjectID)
	assert.Equal(t, "test/svc/app1", stats[0].Path)
	assert.Equal(t, 1, stats[0].Runs, "the completed runs")
	assert.Equal(t, 1, stats[0].Incomplete)
	assert.Equal(t, "test/svc/app2", stats[1].Path)
}

// TestLocalStoreIsCreatedOnFirstWrite pins the create of a missing or empty
// local store before the first call writing a run, through the alias so
// plakar reads the passphrase_cmd, and no create for a store holding
// something.
func TestLocalStoreIsCreatedOnFirstWrite(t *testing.T) {
	writes := map[string]func(e testEnv) error{
		"backup": func(e testEnv) error {
			return e.store.Backup(context.Background(), run1, "fs#1", e.src, e.src, nil)
		},
		"backup stream": func(e testEnv) error {
			return e.store.BackupStream(context.Background(), run1, "config", strings.NewReader("x"))
		},
		"complete": func(e testEnv) error {
			return e.store.Complete(context.Background(), run1)
		},
	}
	for name, write := range writes {
		e := newTestEnv(t)
		require.NoError(t, write(e), name)
		l := calls(t)
		require.Len(t, l, 2, name)
		assert.Equal(t, e.argv("@store", "create"), l[0], name)
		assert.Equal(t, "backup", l[1][len(e.argv("@store"))], name)

		resetFake(t)
		e.createStore(t)
		require.NoError(t, write(e), name)
		require.Len(t, calls(t), 1, "%s: no create for a store holding something", name)
	}
}

// TestReadsNeverCreateTheStore pins a local store missing, or empty as the
// mount point of a shared directory not mounted, read as a store holding no
// run, with no plakar call: a listing, a status, a check, a restore or a
// deletion never creates a store.
func TestReadsNeverCreateTheStore(t *testing.T) {
	ctx := context.Background()
	for _, state := range []string{"missing", "empty"} {
		e := newTestEnv(t)
		if state == "empty" {
			require.NoError(t, os.MkdirAll(e.storePath(), 0700))
		}
		runs, err := e.store.Runs(ctx, backup.RunFilter{})
		require.NoError(t, err, state)
		assert.Empty(t, runs, state)

		stats, err := e.store.ObjectStats(ctx)
		require.NoError(t, err, state)
		assert.Empty(t, stats, state)

		assert.NoError(t, e.store.Check(ctx, backup.RunFilter{ID: "r1"}), "%s: no run to check", state)

		err = e.store.Check(ctx, backup.RunFilter{})
		require.Error(t, err, state)
		assert.Contains(t, err.Error(), "no store at "+e.storePath()+" to check", state)

		assert.ErrorIs(t, e.store.Restore(ctx, run1, "fs#1", e.dst), backup.ErrNotFound, state)

		run := run1
		run.Members = []backup.Member{{Name: "config", Kind: backup.MemberStream}}
		_, err = e.store.Cat(ctx, run, "config")
		assert.ErrorIs(t, err, backup.ErrNotFound, state)

		assert.NoError(t, e.store.Delete(ctx, []backup.Run{run1}), "%s: no run to delete", state)

		assert.Empty(t, calls(t), "%s: no create, nor any other call", state)
		entries, err := os.ReadDir(e.storePath())
		if state == "missing" {
			assert.ErrorIs(t, err, os.ErrNotExist)
		} else {
			require.NoError(t, err)
			assert.Empty(t, entries)
		}
	}
}

// TestPrivateFilesAreOwnerOnly pins the 0700 directories and the 0600
// stores.yml plakar store add would write.
func TestPrivateFilesAreOwnerOnly(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	require.NoError(t, e.store.Check(context.Background(), backup.RunFilter{}))
	for _, dir := range []string{e.dirs.config, e.dirs.cache, e.dirs.data} {
		fi, err := os.Stat(dir)
		require.NoError(t, err, dir)
		assert.True(t, fi.IsDir(), dir)
		assert.Equal(t, os.FileMode(0700), fi.Mode().Perm(), dir)
	}
	stores := filepath.Join(e.dirs.config, "stores.yml")
	fi, err := os.Stat(stores)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), fi.Mode().Perm())
	b, err := os.ReadFile(stores)
	require.NoError(t, err)
	want := strings.TrimPrefix(string(fixture(t, "106-stores-yml-passphrase-cmd.txt")), "add=0\n")
	want = strings.ReplaceAll(want, "fs:///tmp/f/store", "fs://"+e.storePath())
	assert.Equal(t, want, string(b), "what plakar store add writes with the passphrase_cmd")
}

// TestPlakarConfigurationIsWrittenWhole pins sources.yml and destinations.yml
// beside stores.yml: plakar empties stores.yml on its first call without
// them.
func TestPlakarConfigurationIsWrittenWhole(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	require.NoError(t, e.store.Check(context.Background(), backup.RunFilter{}))
	for _, name := range []string{"sources.yml", "destinations.yml"} {
		_, err := os.Stat(filepath.Join(e.dirs.config, name))
		assert.NoError(t, err, name)
	}
}

// TestTheSecurityCheckIsDisabledAgainWithANewCache pins the marker of the
// security check disabled kept in the cache, beside the record of plakar:
// disabled once per store, and again once the cache is removed, which takes
// the record of plakar with it.
func TestTheSecurityCheckIsDisabledAgainWithANewCache(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	calls := 0
	disableSecurityCheck = func(dirs) error { calls++; return nil }
	check := func() {
		s := newStore(t, fmt.Sprintf("[store#lab]\ntype = plakar\nurl = %s\npassphrase = from system/sec/backup key passphrase\n", e.storePath()))
		require.NoError(t, s.Check(context.Background(), backup.RunFilter{}))
	}
	check()
	check()
	assert.Equal(t, 1, calls, "once per store")
	assert.FileExists(t, filepath.Join(e.dirs.cache, securityCheckMarker))
	assert.NoFileExists(t, filepath.Join(e.dirs.config, securityCheckMarker))
	require.NoError(t, os.RemoveAll(e.dirs.cache))
	check()
	assert.Equal(t, 2, calls, "again with a new cache")
}

// TestStoresOfOneNameKeepTheirOwnDirectories pins the directories of a
// store of a sec apart from the ones of the store of the same name of the
// cluster configuration.
func TestStoresOfOneNameKeepTheirOwnDirectories(t *testing.T) {
	e := newTestEnv(t)
	s := newStore(t, "[store#lab]\ntype = plakar\nurl = sftp://u@h/x\n")
	p, err := naming.ParsePath("test/sec/backup")
	require.NoError(t, err)
	s.SetOwner(p)
	assert.Equal(t, filepath.Join(rawconfig.Paths.Run, "store", "plakar", "node", "lab"), e.dirs.config)
	d, err := s.dirs()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(rawconfig.Paths.Run, "store", "plakar", "sec", "test", "backup", "lab"), d.config)
	assert.True(t, e.store.IsLocal())
	assert.False(t, s.IsLocal())
}

// TestUnresolvedStoreIsRetriedOnce pins the stores.yml written again and
// the single retry when plakar cannot resolve the store, and no third try.
func TestUnresolvedStoreIsRetriedOnce(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	require.NoError(t, e.store.Check(context.Background(), backup.RunFilter{}))
	stores := filepath.Join(e.dirs.config, "stores.yml")
	before := inode(t, stores)
	resetFake(t)
	setFake(t, "status.1", []byte("1"))
	setFake(t, "stderr.1", fixture(t, "42-missing-store.txt"))
	require.NoError(t, e.store.Check(context.Background(), backup.RunFilter{}))
	l := calls(t)
	require.Len(t, l, 2, "the call is retried once")
	assert.Equal(t, l[0], l[1])
	assert.NotEqual(t, before, inode(t, stores), "stores.yml written again between the two calls")

	resetFake(t)
	setFake(t, "status", []byte("1"))
	setFake(t, "stderr", fixture(t, "42-missing-store.txt"))
	err := e.store.Check(context.Background(), backup.RunFilter{})
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "plakar check: "), err.Error())
	assert.True(t, strings.HasSuffix(err.Error(), ": plakar: could not resolve repository: @nosuch"), err.Error())
	assert.Len(t, calls(t), 2, "no third try")
}

// inode tells a file written in place from a file replaced.
func inode(t *testing.T, path string) uint64 {
	t.Helper()
	fi, err := os.Stat(path)
	require.NoError(t, err)
	return fi.Sys().(*syscall.Stat_t).Ino
}

// TestExitErrorCarriesTheStderr pins the verb, the exit error and the
// stderr of a failed call, and no retry on an error other than unresolved.
func TestExitErrorCarriesTheStderr(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	setFake(t, "status", []byte("77"))
	setFake(t, "stderr", []byte("plakar: failed to unlock repository\n"))
	err := e.store.Check(context.Background(), backup.RunFilter{})
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "plakar check: "), err.Error())
	assert.True(t, strings.HasSuffix(err.Error(), ": plakar: failed to unlock repository"), err.Error())
	assert.Contains(t, err.Error(), "exit code 77")
	assert.Len(t, calls(t), 1, "no retry on another error")
}

// TestPassphraseIsNeverInArgvEnvFilesOrLogs pins the passphrase_cmd as the
// only way the passphrase reaches plakar: no -keyfile, no PLAKAR_PASSPHRASE,
// no file of the store holding it, no log line holding it.
func TestPassphraseIsNeverInArgvEnvFilesOrLogs(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	var buf bytes.Buffer
	e.store.SetLoggerForTest(plog.NewLogger(zerolog.New(&buf).Level(zerolog.TraceLevel)))
	require.NoError(t, e.store.Backup(context.Background(), run1, "fs#1", e.src, e.src, nil))
	assert.Contains(t, buf.String(), "-datadir "+e.dirs.data+" at @store backup", "the command line is logged")
	assert.NotContains(t, buf.String(), "-keyfile")
	assert.NotContains(t, buf.String(), passphrase, "never the passphrase")
	for _, name := range []string{"calls", "env.1"} {
		b, err := os.ReadFile(filepath.Join(fakeDir, name))
		require.NoError(t, err)
		assert.NotContains(t, string(b), passphrase, name)
		assert.NotContains(t, string(b), "-keyfile", name)
	}
	env, err := os.ReadFile(filepath.Join(fakeDir, "env.1"))
	require.NoError(t, err)
	assert.NotContains(t, string(env), "PLAKAR_PASSPHRASE")
	for _, dir := range []string{e.dirs.config, e.dirs.cache, e.dirs.data} {
		require.NoError(t, filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			b, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.NotContains(t, string(b), passphrase, path)
			return nil
		}))
	}
	_, err = os.Stat(filepath.Join(e.dirs.config, "passphrase"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

// TestPassphraseCmdRunsOmDecodeQuoted pins the passphrase_cmd of stores.yml,
// read back as yaml and run by /bin/sh as plakar does: om gets the sec and
// the key as single words, an om path holding a space and a quote included,
// and what om prints is what plakar reads.
func TestPassphraseCmdRunsOmDecodeQuoted(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	dir := filepath.Join(t.TempDir(), "o'm dir")
	require.NoError(t, os.MkdirAll(dir, 0755))
	om := filepath.Join(dir, "om")
	args := filepath.Join(dir, "args")
	require.NoError(t, os.WriteFile(om, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$(dirname \"$0\")/args\"\necho printed-by-om\n"), 0755))
	t.Setenv(executable.ExecutableKey, om)
	require.NoError(t, e.store.Check(context.Background(), backup.RunFilter{}))

	b, err := os.ReadFile(filepath.Join(e.dirs.config, "stores.yml"))
	require.NoError(t, err)
	var cfg struct {
		Stores map[string]map[string]string `yaml:"stores"`
	}
	require.NoError(t, yaml.Unmarshal(b, &cfg))
	cmd := cfg.Stores["store"]["passphrase_cmd"]
	require.NotEmpty(t, cmd)
	out, err := exec.Command("/bin/sh", "-c", cmd).Output()
	require.NoError(t, err)
	assert.Equal(t, "printed-by-om\n", string(out))
	got, err := os.ReadFile(args)
	require.NoError(t, err)
	assert.Equal(t, "system/sec/backup\nkey\ndecode\n--\npassphrase\n", string(got))
}

// TestPassphraseCmdNamesTheKeyAfterADoubleDash pins the -- before the key
// name: om reads a key named as a flag as a name, not as a flag.
func TestPassphraseCmdNamesTheKeyAfterADoubleDash(t *testing.T) {
	newTestEnv(t)
	p, err := naming.ParsePath("system/sec/backup")
	require.NoError(t, err)
	cmd, err := passphraseCmd(datarecv.KeyMeta{Path: p, Key: "--local"})
	require.NoError(t, err)
	assert.Equal(t, "/usr/bin/om system/sec/backup key decode -- --local", cmd)
}

// TestPassphraseOfOneLineOnly pins the passphrases plakar can read from the
// single line passphrase_cmd prints, and errors never holding the passphrase.
func TestPassphraseOfOneLineOnly(t *testing.T) {
	for _, s := range []string{"s3cret", "s3cret\n"} {
		assert.NoError(t, checkOneLine("passphrase", []byte(s)), "%q", s)
	}
	for _, s := range []string{"", "\n", "s3cret\n\n", "s3c\nret", "s3cret\r\n", "s3c\rret"} {
		err := checkOneLine("passphrase", []byte(s))
		if assert.Error(t, err, "%q", s) {
			assert.NotContains(t, err.Error(), "s3c", "%q", s)
		}
	}
}

// TestUnreadablePassphraseIsRefusedBeforeAnyCall pins the refusal of a key
// plakar cannot read from one line, with no plakar call and no passphrase
// in the error.
func TestUnreadablePassphraseIsRefusedBeforeAnyCall(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	p, err := naming.ParsePath("system/sec/backup")
	require.NoError(t, err)
	sec, err := object.NewSec(p)
	require.NoError(t, err)
	for key, s := range map[string]string{"empty": "", "lines": "s3c\nret"} {
		require.NoError(t, sec.AddKey(key, []byte(s)))
		store := newStore(t, fmt.Sprintf("[store#lab]\ntype = plakar\nurl = %s\npassphrase = from system/sec/backup key %s\n", e.storePath(), key))
		err := store.Check(context.Background(), backup.RunFilter{})
		require.Error(t, err, key)
		assert.Contains(t, err.Error(), "passphrase: ", key)
		assert.NotContains(t, err.Error(), "s3c", key)
	}
	assert.Empty(t, calls(t))
}

// TestMissingSecIsRefusedBeforeAnyCall pins the refusal of a passphrase
// naming a sec that does not exist.
func TestMissingSecIsRefusedBeforeAnyCall(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	s := newStore(t, fmt.Sprintf("[store#lab]\ntype = plakar\nurl = %s\npassphrase = from system/sec/nosuch key passphrase\n", e.storePath()))
	err := s.Check(context.Background(), backup.RunFilter{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "passphrase: ")
	assert.Empty(t, calls(t))
}

// TestMissingBinaryIsRefused pins the error of a node where the
// capabilities scan found no plakar.
func TestMissingBinaryIsRefused(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	getPath = func(string) string { return "" }
	t.Cleanup(func() { getPath = capabilities.GetPath })
	err := e.store.Check(context.Background(), backup.RunFilter{})
	assert.ErrorIs(t, err, errPlakarNotInstalled)
}

// TestCapabilitiesNeedPlakarInThePath pins the driver capability and the
// binary path published only when plakar is in the PATH.
func TestCapabilitiesNeedPlakarInThePath(t *testing.T) {
	t.Setenv("PATH", fakeDir)
	caps, err := capabilitiesScanner(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"drivers.store.plakar", "node.x.path.plakar=" + filepath.Join(fakeDir, "plakar")}, caps)

	t.Setenv("PATH", t.TempDir())
	caps, err = capabilitiesScanner(context.Background())
	require.NoError(t, err)
	assert.Empty(t, caps, "no plakar in the PATH, no capability")
}
