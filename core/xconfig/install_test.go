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
