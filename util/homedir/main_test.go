package homedir

import (
	"os/user"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpand(t *testing.T) {
	t.Setenv("HOME", "/home/alice")
	for path, want := range map[string]string{
		"":                      "",
		"/etc/hosts":            "/etc/hosts",
		"relative/file":         "relative/file",
		"~":                     "/home/alice",
		"~/":                    "/home/alice",
		"~/.config/opensvc/ctx": "/home/alice/.config/opensvc/ctx",
		"~/a/../b":              "/home/alice/b",
		"/not/~/at/the/start":   "/not/~/at/the/start",
	} {
		got, err := Expand(path)
		require.NoError(t, err, path)
		assert.Equal(t, want, got, path)
	}
}

// The home of another user is not looked up.
func TestExpandOtherUser(t *testing.T) {
	t.Setenv("HOME", "/home/alice")
	_, err := Expand("~bob/x")
	assert.Error(t, err)
}

// $HOME is read on each call: a test setting it needs no cache reset.
func TestExpandFollowsHome(t *testing.T) {
	t.Setenv("HOME", "/home/alice")
	got, err := Expand("~/x")
	require.NoError(t, err)
	assert.Equal(t, "/home/alice/x", got)

	t.Setenv("HOME", "/home/bob")
	got, err = Expand("~/x")
	require.NoError(t, err)
	assert.Equal(t, "/home/bob/x", got)
}

// With no $HOME, the home of the user database is used.
func TestExpandWithoutHome(t *testing.T) {
	u, err := user.Current()
	require.NoError(t, err)
	t.Setenv("HOME", "")
	got, err := Expand("~/x")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(u.HomeDir, "x"), got)
}
