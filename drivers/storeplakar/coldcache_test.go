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

// welcome is what plakar prints on its standard output on a cache directory
// it has not run on yet, before the lines of the packages it unpacks.
const welcome = "Welcome to plakar !\n\nBy default, plakar checks for security updates on the releases feed once every 24h.\nIt will notify you if there are important updates that you need to install.\n\nIf you prefer to watch yourself, you can disable this permanently by running:\n\n\tplakar -disable-security-check\n\nIf you change your mind, run:\n\n\tplakar -enable-security-check\n\nEOT\n"

// TestFirstCallReadiesTheCache pins the plakar version run before the first
// call of a driver, with the directories of the store, its output dropped:
// the lines plakar prints unpacking its packages reach neither a listing
// nor the content of a member. It runs once per driver.
func TestFirstCallReadiesTheCache(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	warmUp = runWarmUp
	version := []string{"-configdir", e.dirs.config, "-cachedir", e.dirs.cache, "-datadir", e.dirs.data, "version"}

	setFake(t, "reply.1", []byte(welcome+unpacked+fakeVersion))
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

// TestPlakarOfAnotherReleaseIsRefusedBeforeAnyStoreCall pins the version
// plakar tells in the call readying the cache held to the releases the
// driver reads the outputs of: the 1.1 line from 1.1.8, read after the lines
// plakar prints on a cache not ready. Another release, or an output naming
// none, fails the call and the next ones of the driver, with the version
// found and the ones supported, after one plakar version and no call of the
// store.
func TestPlakarOfAnotherReleaseIsRefusedBeforeAnyStoreCall(t *testing.T) {
	const supported = "the driver supports plakar >= 1.1.8 and < 1.2.0"
	cases := []struct {
		name  string
		reply string
		err   string
	}{
		{"the oldest supported", "plakar/v1.1.8\n", ""},
		{"a later release of the line", "plakar/v1.1.12\n", ""},
		{"a release candidate of a later release of the line", "plakar/v1.1.9-rc.2\n", ""},
		{"after the welcome and the packages unpacked", welcome + unpacked + "plakar/v1.1.8\n", ""},
		{"with no newline", "plakar/v1.1.8", ""},
		{"too old", "plakar/v1.1.7\n", "unsupported plakar version v1.1.7: " + supported},
		{"too old, of the previous line", "plakar/v1.0.6\n", "unsupported plakar version v1.0.6: " + supported},
		{"too old, after the packages unpacked", welcome + unpacked + "plakar/v1.1.7\n", "unsupported plakar version v1.1.7: " + supported},
		{"a pre-release of the oldest supported", "plakar/v1.1.8-rc.1\n", "unsupported plakar version v1.1.8-rc.1: " + supported},
		{"too new", "plakar/v1.2.0\n", "unsupported plakar version v1.2.0: " + supported},
		{"too new, a pre-release", "plakar/v1.2.0-beta.1\n", "unsupported plakar version v1.2.0-beta.1: " + supported},
		{"too new, of the next major", "plakar/v2.1.8\n", "unsupported plakar version v2.1.8: " + supported},
		{"unparsable", "plakar version 1.1.8\n", `unsupported plakar: plakar version prints no plakar/v<major>.<minor>.<patch> line, its output ending with "plakar version 1.1.8": ` + supported},
		{"unparsable, the packages unpacked only", unpacked, `its output ending with "25ff8321: OK ✓ /sftp-storage": ` + supported},
		{"unparsable, a number missing", "plakar/v1.1\n", `its output ending with "plakar/v1.1": ` + supported},
		{"unparsable, empty", "", `its output ending with "": ` + supported},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newTestEnv(t)
			e.createStore(t)
			warmUp = runWarmUp
			setFake(t, "reply.1", []byte(tc.reply))
			if tc.err == "" {
				require.NoError(t, e.store.Check(context.Background(), backup.RunFilter{}))
				require.NoError(t, e.store.Check(context.Background(), backup.RunFilter{}))
				l := calls(t)
				require.Len(t, l, 3, "version, check, check")
				assert.Equal(t, "version", l[0][len(l[0])-1])
				return
			}
			_, err := e.store.Runs(context.Background(), backup.RunFilter{})
			require.Error(t, err)
			assert.ErrorIs(t, err, errUnsupportedPlakar)
			assert.Contains(t, err.Error(), tc.err)
			for _, err := range []error{
				e.store.Check(context.Background(), backup.RunFilter{}),
				e.store.Backup(context.Background(), run1, "fs#1", e.src, e.src, nil),
				e.store.Complete(context.Background(), run1),
				e.store.Delete(context.Background(), []backup.Run{run1}),
			} {
				if assert.Error(t, err) {
					assert.Contains(t, err.Error(), tc.err)
				}
			}
			l := calls(t)
			require.Len(t, l, 1, "one version, asked once, and no call of the store")
			assert.Equal(t, "version", l[0][len(l[0])-1])
			// The store is not the one refused: it keeps its configuration.
			assert.FileExists(t, filepath.Join(e.dirs.config, "stores.yml"))
		})
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
