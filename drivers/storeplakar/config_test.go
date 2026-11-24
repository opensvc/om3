package storeplakar

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/backup"
	"github.com/opensvc/om3/v3/core/naming"
)

func urlStore(t *testing.T, url string) *T {
	t.Helper()
	return newStore(t, "[store#lab]\ntype = plakar\nurl = "+url+"\npassphrase = from system/sec/backup key passphrase\n")
}

// TestURLIsAnAbsolutePathOrASchemeURL pins the urls the driver takes: an
// absolute path and the fs:// url of one, cleaned, are a directory of the
// node, a <scheme>:// url is a location plakar reaches, and anything else,
// which plakar would read from its working directory, is refused.
func TestURLIsAnAbsolutePathOrASchemeURL(t *testing.T) {
	newTestEnv(t)
	cases := map[string]struct {
		location string
		dir      string
		err      string
	}{
		"/srv/plakar":           {location: "fs:///srv/plakar", dir: "/srv/plakar"},
		"/srv//plakar/../store": {location: "fs:///srv/store", dir: "/srv/store"},
		"fs:///srv/plakar":      {location: "fs:///srv/plakar", dir: "/srv/plakar"},
		"fs:///srv/x/../plakar": {location: "fs:///srv/plakar", dir: "/srv/plakar"},
		"sftp://u@h/x":          {location: "sftp://u@h/x"},
		"https://h:9876/":       {location: "https://h:9876/"},
		"s3://h/bucket":         {location: "s3://h/bucket"},
		"fs://srv/plakar":       {err: `url "fs://srv/plakar" is not an absolute path`},
		"fs://":                 {err: `url "fs://" is not an absolute path`},
		"fs://../srv":           {err: `url "fs://../srv" is not an absolute path`},
		"srv/plakar":            {err: `url "srv/plakar" is neither an absolute path nor a <scheme>:// url`},
		"../plakar":             {err: `url "../plakar" is neither an absolute path nor a <scheme>:// url`},
		"@store":                {err: `url "@store" is neither an absolute path nor a <scheme>:// url`},
		"host:/srv/plakar":      {err: `url "host:/srv/plakar" is neither an absolute path nor a <scheme>:// url`},
		"://h/x":                {err: `url "://h/x" is neither an absolute path nor a <scheme>:// url`},
	}
	for url, tc := range cases {
		location, dir, err := urlStore(t, url).location()
		if tc.err != "" {
			if assert.Error(t, err, url) {
				assert.Equal(t, tc.err, err.Error(), url)
			}
			continue
		}
		require.NoError(t, err, url)
		assert.Equal(t, tc.location, location, url)
		assert.Equal(t, tc.dir, dir, url)
	}
	_, _, err := newStore(t, "[store#lab]\ntype = plakar\n").location()
	assert.EqualError(t, err, "no url")
}

// TestIsLocalFailsClosed pins a url the driver refuses reported local: the
// guard keeping the stores of a sec off the directories of the node refuses
// what it cannot tell remote.
func TestIsLocalFailsClosed(t *testing.T) {
	newTestEnv(t)
	for url, want := range map[string]bool{
		"/srv/plakar":      true,
		"fs:///srv/plakar": true,
		"fs://srv/plakar":  true,
		"srv/plakar":       true,
		"../../etc":        true,
		"sftp://u@h/x":     false,
		"https://h/":       false,
	} {
		assert.Equal(t, want, urlStore(t, url).IsLocal(), url)
	}
	assert.True(t, newStore(t, "[store#lab]\ntype = plakar\n").IsLocal(), "no url")
}

// TestRefusedURLIsRefusedBeforeAnyCall pins the url error of every call, a
// read or a write, with no plakar call.
func TestRefusedURLIsRefusedBeforeAnyCall(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	s := urlStore(t, "srv/plakar")
	want := `url "srv/plakar" is neither an absolute path nor a <scheme>:// url`
	_, err := s.Runs(ctx, backup.RunFilter{})
	assert.ErrorContains(t, err, want)
	_, err = s.ObjectStats(ctx)
	assert.ErrorContains(t, err, want)
	assert.ErrorContains(t, s.Check(ctx, backup.RunFilter{}), want)
	assert.ErrorContains(t, s.Restore(ctx, run1, "fs#1", e.dst), want)
	assert.ErrorContains(t, s.Backup(ctx, run1, "fs#1", e.src, e.src, nil), want)
	assert.ErrorContains(t, s.Complete(ctx, run1), want)
	assert.ErrorContains(t, s.Delete(ctx, []backup.Run{run1}), want)
	assert.Empty(t, calls(t))
}

// TestStoreNameIsOnePathElement pins the refusal of a store whose name
// would move its private directories elsewhere on the node, in the cluster
// configuration or in a sec, before any call and before any directory is
// made.
func TestStoreNameIsOnePathElement(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	p, err := naming.ParsePath("test/sec/backup")
	require.NoError(t, err)
	for _, name := range []string{"", ".", "..", "../x", "a/b", "/etc", `a\b`, "../../../../etc/cron.d"} {
		for _, owner := range []naming.Path{{}, p} {
			drv := backup.GetDriver("plakar")
			require.NotNil(t, drv)
			drv.SetName(backup.SectionPrefix + name)
			drv.SetConfig(e.store.Config())
			drv.SetOwner(owner)
			s := drv.(*T)
			_, err := s.dirs()
			if assert.Error(t, err, "%q", name) {
				assert.Contains(t, err.Error(), "invalid store name", "%q", name)
			}
			assert.Error(t, s.Check(context.Background(), backup.RunFilter{}), "%q", name)
			assert.Error(t, s.Backup(context.Background(), run1, "fs#1", e.src, e.src, nil), "%q", name)
			_, err = s.Runs(context.Background(), backup.RunFilter{})
			assert.Error(t, err, "%q", name)
		}
	}
	assert.Empty(t, calls(t))
	for _, name := range []string{"lab", "lab.2", "a-b_c"} {
		assert.NoError(t, validName(name), name)
	}
}
