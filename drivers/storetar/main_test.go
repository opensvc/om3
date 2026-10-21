package storetar

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/backup"
	"github.com/opensvc/om3/v3/core/xconfig"
)

func newStore(t *testing.T, url string) *T {
	t.Helper()
	cfg, err := xconfig.NewObject("", []byte("[store#t1]\ntype = tar\nurl = "+url+"\n"))
	require.NoError(t, err)
	drv := backup.GetDriver("tar")
	require.NotNil(t, drv, "the tar driver registers itself")
	drv.SetName("t1")
	drv.SetConfig(cfg)
	return drv.(*T)
}

func writeFile(t *testing.T, p, content string, mode os.FileMode) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0755))
	require.NoError(t, os.WriteFile(p, []byte(content), mode))
}

func newRun(id string, tm time.Time) backup.Run {
	return backup.Run{ID: id, Path: "ns1/svc/svc1", ObjectID: "oid", Node: "n1", CreatedAt: tm}
}

func TestBackupRestore(t *testing.T) {
	ctx := context.Background()
	storeDir := t.TempDir()
	s := newStore(t, "file://"+storeDir)
	assert.Equal(t, "t1", s.Ref())

	src := t.TempDir()
	writeFile(t, filepath.Join(src, "a.txt"), "a", 0640)
	writeFile(t, filepath.Join(src, "sub", "b.txt"), "b", 0600)
	writeFile(t, filepath.Join(src, "tmp", "junk"), "junk", 0600)
	writeFile(t, filepath.Join(src, "sub", "c.log"), "log", 0600)
	require.NoError(t, os.Symlink("a.txt", filepath.Join(src, "link")))
	require.NoError(t, os.Link(filepath.Join(src, "a.txt"), filepath.Join(src, "hard")))
	require.NoError(t, os.Chmod(filepath.Join(src, "sub"), 0750))

	run := newRun("run1", time.Date(2026, 10, 20, 2, 0, 0, 0, time.UTC))
	require.NoError(t, s.Backup(ctx, run, "fs#1", src, []string{"/tmp", "*.log"}))
	require.NoError(t, s.BackupStream(ctx, run, backup.MemberConfig, strings.NewReader("[DEFAULT]\n")))

	runs, err := s.Runs(ctx, backup.RunFilter{})
	require.NoError(t, err)
	assert.Empty(t, runs, "a run not completed is not listed")
	require.NoError(t, s.Complete(ctx, run))

	runs, err = s.Runs(ctx, backup.RunFilter{})
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.False(t, runs[0].CompletedAt.IsZero())
	assert.Equal(t, "run1", runs[0].ID)
	assert.Equal(t, "ns1/svc/svc1", runs[0].Path)
	assert.Equal(t, "n1", runs[0].Node)
	assert.Equal(t, []string{"fs#1", "config"}, runs[0].MemberNames())
	assert.True(t, run.CreatedAt.Equal(runs[0].CreatedAt))

	dst := t.TempDir()
	require.NoError(t, s.Restore(ctx, runs[0], "fs#1", dst))
	b, err := os.ReadFile(filepath.Join(dst, "sub", "b.txt"))
	require.NoError(t, err)
	assert.Equal(t, "b", string(b))
	info, err := os.Stat(filepath.Join(dst, "a.txt"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0640), info.Mode().Perm())
	info, err = os.Stat(filepath.Join(dst, "sub"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0750), info.Mode().Perm())
	link, err := os.Readlink(filepath.Join(dst, "link"))
	require.NoError(t, err)
	assert.Equal(t, "a.txt", link)
	b, err = os.ReadFile(filepath.Join(dst, "hard"))
	require.NoError(t, err)
	assert.Equal(t, "a", string(b))
	assert.NoFileExists(t, filepath.Join(dst, "tmp", "junk"), "an anchored exclude")
	assert.NoFileExists(t, filepath.Join(dst, "sub", "c.log"), "a name exclude at any depth")

	rc, err := s.Cat(ctx, runs[0], backup.MemberConfig)
	require.NoError(t, err)
	b, err = io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	assert.Equal(t, "[DEFAULT]\n", string(b))

	require.NoError(t, s.Restore(ctx, runs[0], backup.MemberConfig, dst))
	b, err = os.ReadFile(filepath.Join(dst, backup.MemberConfig))
	require.NoError(t, err)
	assert.Equal(t, "[DEFAULT]\n", string(b), "a streamed member restores as a file named after it")

	assert.NoError(t, s.Check(ctx, backup.RunFilter{}))
	assert.Error(t, s.Restore(ctx, runs[0], "fs#2", dst), "a member the run has not")
}

// A restore over an existing tree replaces what the archive holds, a symlink
// where a file is restored included, and never writes through it.
func TestRestoreOverExisting(t *testing.T) {
	ctx := context.Background()
	s := newStore(t, t.TempDir())
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "a.txt"), "new", 0600)
	run := newRun("run1", time.Now())
	require.NoError(t, s.Backup(ctx, run, "fs#1", src, nil))
	run.Members = []backup.Member{{Name: "fs#1", Kind: backup.MemberDir}}

	outside := filepath.Join(t.TempDir(), "outside")
	writeFile(t, outside, "untouched", 0600)
	dst := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(dst, "a.txt")))
	require.NoError(t, s.Restore(ctx, run, "fs#1", dst))
	b, err := os.ReadFile(outside)
	require.NoError(t, err)
	assert.Equal(t, "untouched", string(b))
	b, err = os.ReadFile(filepath.Join(dst, "a.txt"))
	require.NoError(t, err)
	assert.Equal(t, "new", string(b))
}

// An archive whose entries point out of the target writes nothing out of it.
func TestRestoreRefusesEscapes(t *testing.T) {
	ctx := context.Background()
	storeDir := t.TempDir()
	s := newStore(t, storeDir)
	run := newRun("evil", time.Now())
	run.Members = []backup.Member{{Name: "fs#1", Kind: backup.MemberDir}}
	dir, err := s.runDir(run)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(dir, 0700))

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "esc", Typeflag: tar.TypeSymlink, Linkname: "/", Mode: 0777}))
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "esc/pwned", Typeflag: tar.TypeReg, Mode: 0600, Size: 1}))
	_, _ = tw.Write([]byte("x"))
	require.NoError(t, tw.Close())
	require.NoError(t, zw.Close())
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fs#1.tar.gz"), buf.Bytes(), 0600))

	dst := t.TempDir()
	assert.Error(t, s.Restore(ctx, run, "fs#1", dst))
	assert.NoFileExists(t, "/pwned")
}

func TestRunsFilterAndDelete(t *testing.T) {
	ctx := context.Background()
	s := newStore(t, t.TempDir())
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "a"), "a", 0600)
	t0 := time.Date(2026, 10, 20, 2, 0, 0, 0, time.UTC)
	for i, id := range []string{"7c0e41d2-1", "7c0e9999-2", "b93f0a17-3"} {
		run := newRun(id, t0.Add(time.Duration(i)*time.Hour))
		require.NoError(t, s.Backup(ctx, run, "fs#1", src, nil))
		require.NoError(t, s.Complete(ctx, run))
	}
	other := newRun("aaaa-4", t0)
	other.Path = "ns2/svc/web"
	require.NoError(t, s.Backup(ctx, other, "fs#1", src, nil))
	require.NoError(t, s.Complete(ctx, other))

	runs, err := s.Runs(ctx, backup.RunFilter{Path: "ns1/svc/svc1"})
	require.NoError(t, err)
	require.Len(t, runs, 3)
	assert.Equal(t, "7c0e41d2-1", runs[0].ID, "oldest first")

	runs, err = s.Runs(ctx, backup.RunFilter{})
	require.NoError(t, err)
	assert.Len(t, runs, 4)

	_, err = backup.FindRun(ctx, s, "ns1/svc/svc1", "7c0e")
	assert.ErrorIs(t, err, backup.ErrAmbiguous)
	run, err := backup.FindRun(ctx, s, "ns1/svc/svc1", "7c0e4")
	require.NoError(t, err)
	assert.Equal(t, "7c0e41d2-1", run.ID)
	run, err = backup.FindRun(ctx, s, "ns1/svc/svc1", "")
	require.NoError(t, err)
	assert.Equal(t, "b93f0a17-3", run.ID, "no id is the latest run")
	_, err = backup.FindRun(ctx, s, "ns1/svc/svc1", "ffff")
	assert.ErrorIs(t, err, backup.ErrNotFound)

	require.NoError(t, s.Delete(ctx, []backup.Run{run}))
	runs, err = s.Runs(ctx, backup.RunFilter{Path: "ns1/svc/svc1"})
	require.NoError(t, err)
	assert.Len(t, runs, 2)
}

// A run whose member was being written when it stopped lists the members
// written whole only.
func TestInterruptedMemberIsNotListed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := newStore(t, t.TempDir())
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "a"), "a", 0600)
	run := newRun("run1", time.Now())
	require.NoError(t, s.Backup(ctx, run, "fs#1", src, nil))
	cancel()
	assert.Error(t, s.Backup(ctx, run, "fs#2", src, nil))
	runs, err := s.Runs(context.Background(), backup.RunFilter{})
	require.NoError(t, err)
	assert.Empty(t, runs, "an interrupted run is not listed")
	runs, err = s.Runs(context.Background(), backup.RunFilter{Incomplete: true})
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, []string{"fs#1"}, runs[0].MemberNames())
	dir, err := s.runDir(run)
	require.NoError(t, err)
	assert.NoFileExists(t, filepath.Join(dir, "fs#2.tar.gz"+tmpSuffix))
}

func TestCheckFindsCorruption(t *testing.T) {
	ctx := context.Background()
	s := newStore(t, t.TempDir())
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "a"), strings.Repeat("a", 4096), 0600)
	run := newRun("run1", time.Now())
	require.NoError(t, s.Backup(ctx, run, "fs#1", src, nil))
	require.NoError(t, s.Complete(ctx, run))
	dir, err := s.runDir(run)
	require.NoError(t, err)
	p := filepath.Join(dir, "fs#1.tar.gz")
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	b[len(b)/2] ^= 0xff
	require.NoError(t, os.WriteFile(p, b, 0600))
	assert.Error(t, s.Check(ctx, backup.RunFilter{}))
}

func TestRefusals(t *testing.T) {
	ctx := context.Background()
	src := t.TempDir()
	s := newStore(t, filepath.Join(src, "store"))
	assert.ErrorContains(t, s.Backup(ctx, newRun("run1", time.Now()), "fs#1", src, nil), "never inside a head")

	s = newStore(t, t.TempDir())
	assert.Error(t, s.Backup(ctx, newRun("../x", time.Now()), "fs#1", src, nil))
	assert.Error(t, s.Backup(ctx, newRun("run1", time.Now()), "../fs", src, nil))
	assert.Error(t, newStore(t, "relative/dir").Check(ctx, backup.RunFilter{}))
}

func TestWithin(t *testing.T) {
	assert.True(t, within("/srv/a/b", "/srv/a"))
	assert.True(t, within("/srv/a", "/srv/a"))
	assert.False(t, within("/srv/ab", "/srv/a"))
	assert.False(t, within("/srv", "/srv/a"))
}
