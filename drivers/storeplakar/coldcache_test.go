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

// unpacked is what plakar prints on its standard output when it unpacks a
// package into a cache directory not holding it, before what its command
// prints.
const unpacked = "25ff8321: OK ✓ /\n25ff8321: OK ✓ /manifest.yaml\n25ff8321: OK ✓ /sftp-storage\n"

// TestFirstCallReadiesTheCache pins the plakar version run before the first
// call of a driver, with the directories of the store, its output dropped:
// the lines plakar prints unpacking its packages reach neither a listing
// nor the content of a member. It runs once per driver.
func TestFirstCallReadiesTheCache(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	warmUp = runWarmUp
	version := []string{"-configdir", e.dirs.config, "-cachedir", e.dirs.cache, "-datadir", e.dirs.data, "version"}

	setFake(t, "reply.1", []byte(unpacked))
	setFake(t, "reply.2", []byte(
		lsLine(t0.Add(2*time.Minute), "s3", run1, "om3.complete,om3.completed=2026-10-27T10%3A02%3A00Z")+
			lsLine(t0, "s2", run1, "om3.member=config,om3.kind=stream")))
	setFake(t, "reply.3", fixture(t, "33-cat-refs.txt"))
	runs, err := e.store.Runs(context.Background(), backup.RunFilter{})
	require.NoError(t, err, "the lines of the packages unpacked are not read as a listing")
	require.Len(t, runs, 1)
	rc, err := e.store.Cat(context.Background(), runs[0], "config")
	require.NoError(t, err)
	b, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, string(fixture(t, "33-cat-refs.txt")), string(b), "nor as the content of a member")
	l := calls(t)
	require.Len(t, l, 3, "one version, before the first call only")
	assert.Equal(t, version, l[0])
	assert.Equal(t, e.argv("@store", "ls", "-uuid", "-tags"), l[1])
	assert.Equal(t, e.argv("@store", "cat", "s2:/config"), l[2])

	// Another driver, another command or action, readies it again: a
	// package may have been installed since.
	resetFake(t)
	s := urlStore(t, e.storePath())
	require.NoError(t, s.Check(context.Background(), backup.RunFilter{}))
	require.NoError(t, s.Check(context.Background(), backup.RunFilter{}))
	l = calls(t)
	require.Len(t, l, 3)
	assert.Equal(t, version, l[0])
}

// TestFirstCallOfASFTPStoreReadiesTheCacheWithItsSSH pins the environment
// of the store given to the call readying the cache too.
func TestFirstCallOfASFTPStoreReadiesTheCacheWithItsSSH(t *testing.T) {
	e := newCredsEnv(t)
	warmUp = runWarmUp
	t.Setenv("PATH", "/usr/bin:/bin")
	require.NoError(t, e.sftpStore(t).Check(context.Background(), backup.RunFilter{}))
	l := calls(t)
	require.Len(t, l, 2)
	assert.Equal(t, "version", l[0][len(l[0])-1])
	env, err := os.ReadFile(filepath.Join(fakeDir, "env.1"))
	require.NoError(t, err)
	assert.Contains(t, "\n"+string(env), "\nPATH="+e.dirs.bin+":/usr/bin:/bin\n")
}

// TestCacheNotReadiedDoesNotFailTheCall pins a plakar version failing
// logged, the call it precedes run all the same, and tried again before the
// next call.
func TestCacheNotReadiedDoesNotFailTheCall(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	warmUp = runWarmUp
	var logs bytes.Buffer
	e.store.SetLoggerForTest(plog.NewLogger(zerolog.New(&logs).Level(zerolog.InfoLevel)))
	setFake(t, "status.1", []byte("1"))
	setFake(t, "stderr.1", []byte("plakar: cannot create the cache\n"))
	require.NoError(t, e.store.Check(context.Background(), backup.RunFilter{}))
	assert.Contains(t, logs.String(), "warm the cache of plakar up: plakar version: ")
	require.NoError(t, e.store.Check(context.Background(), backup.RunFilter{}))
	require.NoError(t, e.store.Check(context.Background(), backup.RunFilter{}))
	l := calls(t)
	require.Len(t, l, 5, "version failed, check, version, check, check")
	for i, verb := range []string{"version", "check", "version", "check", "check"} {
		assert.Equal(t, verb, l[i][len(l[i])-1], i)
	}
}

// TestWithTheRealPlakarOnAColdCache runs the driver against the plakar of
// the host and the packages its operator installed, on a cache directory
// removed before a listing and before the read of a member: plakar unpacks
// the packages again, printing a line per file, which the listing and the
// member must not hold. It is skipped on a host without plakar, or with no
// package installed.
func TestWithTheRealPlakarOnAColdCache(t *testing.T) {
	bin, err := exec.LookPath("plakar")
	if err != nil {
		t.Skip("no plakar in the PATH")
	}
	packages, _ := filepath.Glob(filepath.Join(defaultDataDir(), "plugins", "*", "*.ptar"))
	if len(packages) == 0 {
		t.Skip("no plakar package installed in " + defaultDataDir())
	}
	e := newTestEnv(t)
	dataDir = defaultDataDir
	disableSecurityCheck = runDisableSecurityCheck
	warmUp = runWarmUp
	getPath = func(string) string { return bin }
	t.Cleanup(func() { getPath = capabilities.GetPath })
	om := filepath.Join(t.TempDir(), "om")
	require.NoError(t, os.WriteFile(om, []byte("#!/bin/sh\necho "+passphrase+"\n"), 0755))
	t.Setenv(executable.ExecutableKey, om)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	content := "[DEFAULT]\nid = o1\n"
	require.NoError(t, e.store.BackupStream(ctx, run1, backup.MemberConfig, strings.NewReader(content)))
	require.NoError(t, e.store.Complete(ctx, run1))

	// The control: on a cache removed, a plakar call prints the lines of
	// the packages it unpacks before its own.
	require.NoError(t, os.RemoveAll(e.dirs.cache))
	require.NoError(t, os.MkdirAll(e.dirs.cache, 0700))
	out, err := exec.Command(bin, "-configdir", e.dirs.config, "-cachedir", e.dirs.cache, "-datadir", defaultDataDir(), "version").Output()
	require.NoError(t, err)
	require.Contains(t, string(out), ": OK ", "plakar prints what it unpacks on its standard output")

	require.NoError(t, os.RemoveAll(e.dirs.cache))
	s := urlStore(t, e.storePath())
	runs, err := s.Runs(ctx, backup.RunFilter{})
	require.NoError(t, err, "a listing on a cold cache")
	require.Len(t, runs, 1)

	require.NoError(t, os.RemoveAll(e.dirs.cache))
	s = urlStore(t, e.storePath())
	b, err := backup.ReadMember(ctx, s, runs[0], backup.MemberConfig)
	require.NoError(t, err)
	assert.Equal(t, content, string(b), "a member read on a cold cache holds its content only")
}
