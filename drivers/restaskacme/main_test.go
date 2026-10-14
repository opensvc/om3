package restaskacme

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/confined"
)

// A sec is named as an install line names a store: a name or ./sec/<name> in
// the namespace of the service, <namespace>/sec/<name> in another.
func TestSecPath(t *testing.T) {
	for ref, want := range map[string]string{
		"web":         "ns1/sec/web",
		"./sec/web":   "ns1/sec/web",
		"ns2/sec/web": "ns2/sec/web",
		"root/sec/ca": "sec/ca",
	} {
		p, err := secPath(ref, "ns1")
		require.NoError(t, err, ref)
		assert.Equal(t, want, p.String(), ref)
	}
	for _, ref := range []string{"ns2/cfg/web", "./svc/web"} {
		_, err := secPath(ref, "ns1")
		assert.Error(t, err, ref)
	}
	p, _ := secPath("web", "ns1")
	assert.Equal(t, naming.KindSec, p.Kind)
}

// The challenge token is written under .well-known/acme-challenge of the
// webroot, readable by the http server, and removed after the challenge.
func TestConfinedWebrootWritesTheToken(t *testing.T) {
	head := t.TempDir()
	tree, err := confined.Open(head)
	require.NoError(t, err)
	defer func() { _ = tree.Close() }()
	w := &confinedWebroot{tree: tree, dir: filepath.Join(head, "haproxy", "acme-challenges")}

	require.NoError(t, w.Present("dev2-oc3.opensvc.com", "tok_en-1", "tok_en-1.thumb"))
	p := filepath.Join(head, "haproxy", "acme-challenges", ".well-known", "acme-challenge", "tok_en-1")
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, "tok_en-1.thumb", string(b))
	st, err := os.Stat(p)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), st.Mode().Perm())

	require.NoError(t, w.CleanUp("dev2-oc3.opensvc.com", "tok_en-1", ""))
	_, err = os.Stat(p)
	assert.True(t, os.IsNotExist(err))

	for _, token := range []string{"", "..", "a/b", `a\b`} {
		assert.Error(t, w.Present("d", token, "x"), token)
	}
}

// A link planted in the volume does not lead the token write out of it.
func TestConfinedWebrootStaysInTheVolume(t *testing.T) {
	head := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(head, "www"), 0o755))
	require.NoError(t, os.Symlink(outside, filepath.Join(head, "www", ".well-known")))
	tree, err := confined.Open(head)
	require.NoError(t, err)
	defer func() { _ = tree.Close() }()
	w := &confinedWebroot{tree: tree, dir: filepath.Join(head, "www")}

	_ = w.Present("d", "tok", "auth")
	entries, err := os.ReadDir(outside)
	require.NoError(t, err)
	assert.Empty(t, entries, "nothing written through the link")
}

// The webroot names a path in a volume or a filesystem of the service, never
// a path of the node.
func TestWebrootSyntax(t *testing.T) {
	for _, s := range []string{"/srv/www", "volume#1:/www", "volume#1/www", "nota#rid:/www"} {
		task := &T{Webroot: s}
		_, _, err := task.http01(context.Background())
		assert.Error(t, err, s)
	}
	task := &T{}
	p, _, err := task.http01(context.Background())
	require.NoError(t, err)
	assert.Nil(t, p, "no webroot, no writer: the secs are not acme ones, or name their own")
}

// A pattern names the secs there are, a sec named one asked: a pattern
// matching none names none, and leaves out a sec of another namespace not
// shared, where a sec named that does not exist fails.
func TestSecsOfPatterns(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)
	sec := func(ns, name, conf string) {
		p := naming.Path{Namespace: ns, Kind: naming.KindSec, Name: name}
		require.NoError(t, os.MkdirAll(filepath.Dir(p.ConfigFile()), 0o755))
		require.NoError(t, os.WriteFile(p.ConfigFile(), []byte(conf), 0o600))
	}
	sec("ns1", "web-a", "")
	sec("ns1", "web-b", "")
	sec("ns1", "other", "")
	sec("ns2", "web-c", "[DEFAULT]\nshare = ns1\n")
	sec("ns2", "web-d", "")

	task := &T{Path: naming.Path{Namespace: "ns1", Kind: naming.KindSvc, Name: "gw"}}
	task.Secs = []string{"./sec/web-*", "ns2/sec/web-*", "web-a", "none-*"}
	l, err := task.secs()
	require.NoError(t, err)
	assert.Equal(t, []string{"ns1/sec/web-a", "ns1/sec/web-b", "ns2/sec/web-c"}, l.StrSlice())

	task.Secs = []string{"web-*", "nope", "ns2/sec/web-d"}
	l, err = task.secs()
	assert.ErrorContains(t, err, "ns1/sec/nope does not exist")
	assert.ErrorContains(t, err, "ns2/sec/web-d is not shared with namespace ns1")
	assert.Equal(t, []string{"ns1/sec/web-a", "ns1/sec/web-b"}, l.StrSlice(), "the secs found are renewed all the same")
}

func TestValidateSecs(t *testing.T) {
	assert.NoError(t, validateSecs("web ./sec/web-* ns2/sec/*.example.com api-*"))
	for _, value := range []string{"./sec/*-*", "./cfg/web-*", "ns2/cfg/web", "./sec/Web-*"} {
		assert.Error(t, validateSecs(value), value)
	}
}
