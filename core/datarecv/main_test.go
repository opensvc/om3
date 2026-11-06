package datarecv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/naming"
)

// A source is taken for a fetch only when the install fetches it: a text the
// install reads from the local filesystem is a local source, whatever prefix
// it wears.
func TestTextHasLocalSource(t *testing.T) {
	for s, want := range map[string]bool{
		"/init/x from ./cfg/{name} key x source https://example.com/app.conf":         false,
		"/init/x from ./cfg/{name} key x source http://example.com/app.conf template": false,
		"/init/x from ./cfg/{name} key x source /etc/app.conf":                        true,
		"/init/x from ./cfg/{name} key x source http:///etc/shadow":                   true,
		"/init/x from ./cfg/{name} key x source https:/etc/shadow":                    true,
		"/init/x from ./cfg/{name} key x source http:etc/shadow":                      true,
		"/init/x from ./cfg/{name} key x mode 0644":                                   false,
		"/etc/ mode 0750\n/init/y from ./cfg/{name} key y source http:///etc/shadow":  true,
		"/init/x from ./sec/{name} key x source random":                               false,
		"/init/x from ./sec/{name} key x source /dev/urandom":                         true,
	} {
		if got := TextHasLocalSource(s); got != want {
			t.Errorf("%q: got %v, want %v", s, got, want)
		}
	}
}

// An environment seeding a key from a file of the node names a local source,
// a fetch or a key generated does not, and a value the grammar refuses is
// judged by the words following "source".
func TestEnvironmentHasLocalSource(t *testing.T) {
	for s, want := range map[string]bool{
		"A=1 B from ./sec/a key k":                                              false,
		"A from ./sec/a key k source random":                                    false,
		"A from ./sec/a key k source https://example.com/x":                     false,
		"A from ./sec/a key k source /etc/shadow":                               true,
		"A from ./sec/a key k source /dev/urandom":                              true,
		"A from ./sec/a key k source http:///etc/shadow":                        true,
		"A=1 B from ./sec/a key k source random C from ./cfg/b key k2":          false,
		"A from ./sec/a key k source random B from ./cfg/b key c source /etc/x": true,
		"A from ./svc/a key k source /etc/shadow":                               true,
		"source=1 A from ./sec/a key k":                                         false,
	} {
		if got := EnvironmentHasLocalSource(s); got != want {
			t.Errorf("%q: got %v, want %v", s, got, want)
		}
	}
}

// The keys an install seeds are the ones of its lines naming a source, the
// key defaulting to the path of the line.
func TestInstallKeySources(t *testing.T) {
	l, err := InstallKeySources([]string{
		"/etc/", "mode", "0750",
		"/a", "from", "./sec/s", "key", "pw", "mode", "0600", "source", "random", "required",
		"/b", "from", "./cfg/c", "source", "https://example.com/b",
		"/c", "from", "./cfg/c", "key", "c",
	}, "dev")
	require.NoError(t, err)
	assert.Equal(t, []keywords.KeySource{
		{Path: naming.Path{Namespace: "dev", Kind: naming.KindSec, Name: "s"}, Key: "pw", Source: "random"},
		{Path: naming.Path{Namespace: "dev", Kind: naming.KindCfg, Name: "c"}, Key: "b", Source: "https://example.com/b"},
	}, l)
}

// A source seeds a key named: a key pattern names none, and a key generated
// is not seeded in the datastores of a pattern, as a datastore made later
// would never have one. An url seeds the datastores a pattern matches.
func TestInstallKeySourcesPatterns(t *testing.T) {
	_, err := InstallKeySources([]string{"/conf/", "from", "./cfg/c", "key", "conf/*", "source", "https://example.com/c"}, "dev")
	require.Error(t, err, "a key pattern")
	assert.Contains(t, err.Error(), "not the keys of a pattern")

	_, err = InstallKeySources([]string{"/certs/*.pem", "from", "./sec/haproxy-*", "key", "fullpem", "source", "random"}, "dev")
	require.Error(t, err, "a key generated in the datastores of a pattern")
	assert.Contains(t, err.Error(), "add it with each datastore")

	_, err = InstallKeySources([]string{"/certs/*.pem", "from", "./sec/haproxy-*", "key", "fullpem", "source", "https://example.com/c"}, "dev")
	assert.NoError(t, err, "an url in the datastores of a pattern")
}
