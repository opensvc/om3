package storeplakar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/backup"
	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/capabilities"
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
	env := testhelper.Setup(t)
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

	e := testEnv{root: env.Root, src: filepath.Join(env.Root, "src"), dst: filepath.Join(env.Root, "dst")}
	require.NoError(t, os.MkdirAll(e.src, 0755))
	e.store = newStore(t, fmt.Sprintf("[store#lab]\ntype = plakar\nurl = %s\npassphrase = from system/sec/backup key passphrase\n", e.storePath()))
	e.dirs = e.store.dirs()
	return e
}

// createStore makes the local store look created, so no create call runs.
func (e testEnv) createStore(t *testing.T) {
	t.Helper()
	require.NoError(t, os.MkdirAll(e.storePath(), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(e.storePath(), "CONFIG"), []byte("x"), 0600))
}

// argv is the command line plakar receives in the environment: the private
// directories and the key file, then the store and the verb.
func (e testEnv) argv(at string, args ...string) []string {
	return append([]string{"-configdir", e.dirs.config, "-cachedir", e.dirs.cache, "-datadir", e.dirs.data, "-keyfile", e.dirs.keyfile(), "at", at}, args...)
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
// inside the directory it would back up, before any call.
func TestBackupRefusesAStoreUnderTheSource(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	err := e.store.Backup(context.Background(), run1, "fs#1", e.root, e.root, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "a store is never inside a head it backs up")
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
		e.argv("@store", "ls", "-uuid", "-tags", "-tag", "om3.run=r1,om3.member=fs%231"),
		e.argv("@store", "restore", "-to", e.dst, "s1"),
	}, calls(t))
	assert.DirExists(t, e.dst)

	resetFake(t)
	err := e.store.Restore(context.Background(), run1, "fs#2", e.dst)
	assert.ErrorIs(t, err, backup.ErrNotFound)
	assert.Len(t, calls(t), 1, "no restore of a member not found")
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
// of each run, its marker too, a run with no snapshot removing nothing.
func TestDeleteRemovesEverySnapshotOfTheRuns(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	setFake(t, "reply.2", fixture(t, "92-rm-apply-nomatch.txt"))
	run2 := run1
	run2.ID = "r2"
	require.NoError(t, e.store.Delete(context.Background(), []backup.Run{run1, run2}))
	assert.Equal(t, [][]string{
		e.argv("@store", "rm", "-apply", "-tag", "om3.run=r1"),
		e.argv("@store", "rm", "-apply", "-tag", "om3.run=r2"),
	}, calls(t))
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

// TestLocalStoreIsCreatedOnFirstUse pins the create of an empty local store
// before the first call, and no create for a store holding something.
func TestLocalStoreIsCreatedOnFirstUse(t *testing.T) {
	e := newTestEnv(t)
	require.NoError(t, e.store.Check(context.Background(), backup.RunFilter{}))
	assert.Equal(t, [][]string{
		e.argv("fs://"+e.storePath(), "create"),
		e.argv("@store", "check"),
	}, calls(t), "create, then check")

	resetFake(t)
	e.createStore(t)
	require.NoError(t, e.store.Check(context.Background(), backup.RunFilter{}))
	require.Len(t, calls(t), 1, "no create for a store holding something")
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
	want := strings.ReplaceAll(string(fixture(t, "70-stores-yml.txt")), "fs:///tmp/fix/store", "fs://"+e.storePath())
	want = strings.ReplaceAll(want, "    lab:", "    store:")
	assert.Equal(t, want, string(b), "what plakar store add writes")
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
	assert.Equal(t, filepath.Join(rawconfig.Paths.Run, "store", "plakar", "sec", "test", "backup", "lab"), s.dirs().config)
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

// TestPassphraseIsNeverInArgvEnvOrLogs pins the key file as the only way
// the passphrase reaches plakar, with the exact bytes of the sec key.
func TestPassphraseIsNeverInArgvEnvOrLogs(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	var buf bytes.Buffer
	e.store.SetLoggerForTest(plog.NewLogger(zerolog.New(&buf).Level(zerolog.TraceLevel)))
	require.NoError(t, e.store.Backup(context.Background(), run1, "fs#1", e.src, e.src, nil))
	assert.Contains(t, buf.String(), "-keyfile "+e.dirs.keyfile(), "the command line is logged with the key file path")
	assert.NotContains(t, buf.String(), passphrase, "never the passphrase")
	for _, name := range []string{"calls", "env.1"} {
		b, err := os.ReadFile(filepath.Join(fakeDir, name))
		require.NoError(t, err)
		assert.NotContains(t, string(b), passphrase, name)
	}
	env, err := os.ReadFile(filepath.Join(fakeDir, "env.1"))
	require.NoError(t, err)
	assert.NotContains(t, string(env), "PLAKAR_PASSPHRASE")
	fi, err := os.Stat(e.dirs.keyfile())
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), fi.Mode().Perm())
	b, err := os.ReadFile(e.dirs.keyfile())
	require.NoError(t, err)
	assert.Equal(t, passphrase, string(b), "the key file holds the exact bytes of the sec key")
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
