package xconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/rawconfig"
)

// A write given a base lands only over it: a file written, created or
// removed meanwhile refuses it, and is left as the other writer left it.
func TestInstallOverBase(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "foo.conf")
	setFile := func(s string) {
		require.NoError(t, os.WriteFile(dst, []byte(s), 0600))
	}
	newSrc := func() string {
		src := filepath.Join(dir, ".foo.conf.tmp")
		require.NoError(t, os.WriteFile(src, []byte("[DEFAULT]\nwritten = true\n"), 0600))
		return src
	}
	content := func() string {
		b, err := os.ReadFile(dst)
		if os.IsNotExist(err) {
			return "<none>"
		}
		require.NoError(t, err)
		return string(b)
	}

	setFile("[DEFAULT]\n")
	base, err := ReadBase(dst)
	require.NoError(t, err)
	require.NoError(t, install(newSrc(), dst, &base))
	assert.Equal(t, "[DEFAULT]\nwritten = true\n", content(), "the file checked against is written over")

	for _, tc := range []struct {
		name   string
		before func()
		after  func()
		left   string
	}{
		{"written meanwhile", func() { setFile("[DEFAULT]\n") }, func() { setFile("[DEFAULT]\nother = 1\n") }, "[DEFAULT]\nother = 1\n"},
		{"created meanwhile", func() { _ = os.Remove(dst) }, func() { setFile("[DEFAULT]\n") }, "[DEFAULT]\n"},
		{"removed meanwhile", func() { setFile("[DEFAULT]\n") }, func() { require.NoError(t, os.Remove(dst)) }, "<none>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.before()
			base, err := ReadBase(dst)
			require.NoError(t, err)
			tc.after()
			assert.ErrorIs(t, install(newSrc(), dst, &base), ErrConfigChanged)
			assert.Equal(t, tc.left, content())
		})
	}

	t.Run("no base", func(t *testing.T) {
		setFile("[DEFAULT]\nother = 1\n")
		require.NoError(t, InstallFile(newSrc(), dst))
		assert.Equal(t, "[DEFAULT]\nwritten = true\n", content())
	})
}

// The lock of a configuration file is kept out of the directory the daemon
// watches.
// What a write does as it lands is done by the write that lands alone: two
// writes racing over the same base, the one refused does nothing, and the
// one whose file is not replaced after all undoes it.
func TestInstallLanding(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "foo.conf")
	newSrc := func() string {
		src := filepath.Join(dir, ".foo.conf.tmp")
		require.NoError(t, os.WriteFile(src, []byte("[DEFAULT]\n"), 0600))
		return src
	}
	var landed, undone, kept int
	done := func(ok bool) {
		if ok {
			kept++
		} else {
			undone++
		}
	}
	landing := func() (func(bool), error) {
		landed++
		return done, nil
	}

	first, err := ReadBase(dst)
	require.NoError(t, err)
	second, err := ReadBase(dst)
	require.NoError(t, err)
	firstLanding, secondLanding := first.Landing(landing), second.Landing(landing)
	require.NoError(t, install(newSrc(), dst, &firstLanding))
	assert.Equal(t, 1, landed, "the write landing runs it")
	assert.Equal(t, 1, kept, "and says it landed")
	assert.ErrorIs(t, install(newSrc(), dst, &secondLanding), ErrConfigChanged)
	assert.Equal(t, 1, landed, "the write refused does not run it")
	assert.Equal(t, 0, undone)

	base, err := ReadBase(dst)
	require.NoError(t, err)
	base = base.Landing(landing)
	assert.Error(t, install(filepath.Join(dir, "missing"), dst, &base))
	assert.Equal(t, 2, landed)
	assert.Equal(t, 1, undone, "the write whose file is not replaced undoes it")

	base, err = ReadBase(dst)
	require.NoError(t, err)
	base = base.Landing(func() (func(bool), error) { landed++; return done, os.ErrPermission })
	assert.ErrorIs(t, install(newSrc(), dst, &base), os.ErrPermission, "an error of the landing refuses the write")
	assert.Equal(t, 2, undone, "and undoes what the landing did before failing")
}

func TestInstallLockPath(t *testing.T) {
	saved := rawconfig.Paths
	t.Cleanup(func() { rawconfig.Paths = saved })
	rawconfig.Paths.Etc = "/etc/opensvc"
	rawconfig.Paths.Lock = "/var/lib/opensvc/lock"
	assert.Equal(t, "/var/lib/opensvc/lock/config/ns1/svc/foo.conf", installLockPath("/etc/opensvc/ns1/svc/foo.conf"))
	assert.Equal(t, "/var/lib/opensvc/lock/config/cluster.conf", installLockPath("/etc/opensvc/cluster.conf"))
	assert.Equal(t, "/tmp/x/.foo.conf.lock", installLockPath("/tmp/x/foo.conf"))
	assert.Equal(t, "/etc/.opensvc.lock", installLockPath("/etc/opensvc"))
}
