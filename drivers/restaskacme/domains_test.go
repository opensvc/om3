package restaskacme

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/key"
)

func TestDomainsFromMatch(t *testing.T) {
	from, err := parseDomainsFrom("volume#1:/haproxy/conf.d/*.cfg")
	require.NoError(t, err)
	assert.Equal(t, "volume#1:/haproxy/conf.d", from.dir)
	for name, want := range map[string]string{
		"www.example.com.cfg": "www.example.com",
		"a.b.c.io.cfg":        "a.b.c.io",
		"README":              "",
		"tcp-smtp.cfg":        "",
		"WWW.example.com.cfg": "",
		"www.example.com":     "",
		".cfg":                "",
		"www.example.1.cfg":   "",
	} {
		got, ok := from.match(name)
		assert.Equal(t, want != "", ok, name)
		if ok {
			assert.Equal(t, want, got, name)
		}
	}
	for _, s := range []string{"volume#1:/haproxy/*/x.cfg", "volume#1:/haproxy/conf.d/x.cfg", "*.cfg", "volume#1:/a/*-*.cfg"} {
		_, err := parseDomainsFrom(s)
		assert.Error(t, err, s)
	}
}

// The sec of a domain is named after it, and after its beginning and a hash
// of it when the name would be longer than an object name is.
func TestDomainSecPath(t *testing.T) {
	pattern, err := parseDomainsTo("./sec/haproxy-*", "ns1")
	require.NoError(t, err)
	p, err := domainSecPath(pattern, "www.example.com")
	require.NoError(t, err)
	assert.Equal(t, "ns1/sec/haproxy-www.example.com", p.String())

	long := strings.Repeat("abcdefghij.", 6) + "example.com"
	p, err = domainSecPath(pattern, long)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(p.Name), maxSecName)
	assert.True(t, strings.HasPrefix(p.Name, "haproxy-abcdefghij.abcdefghij."), p.Name)
	assert.True(t, pattern.MatchPath(p), "the name matches the pattern")
	other, err := domainSecPath(pattern, strings.Repeat("abcdefghij.", 6)+"example.org")
	require.NoError(t, err)
	assert.NotEqual(t, p, other, "two long domains of the same beginning")
}

func TestReady(t *testing.T) {
	ctx := context.Background()
	ok, why := ready(ctx, "localhost", nil)
	assert.False(t, ok)
	assert.Contains(t, why, "no address")
	ok, _ = ready(ctx, "localhost", []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")})
	assert.True(t, ok)
	ok, why = ready(ctx, "localhost", []net.IP{net.ParseIP("192.0.2.1")})
	assert.False(t, ok)
	assert.Contains(t, why, "none of the service addresses 192.0.2.1")
}

// The secs the task created are its own, told by their owner key, and their
// domain by their cn: another sec the pattern matches is not.
func TestCreatedSecsAreOwned(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)

	task := &T{
		Path:      naming.Path{Namespace: "ns1", Kind: naming.KindSvc, Name: "gw"},
		Directory: "letsencrypt-staging",
		Email:     "admin@example.com",
	}
	require.NoError(t, task.SetRID("task#acme"))
	pattern, err := parseDomainsTo("./sec/gw-*", "ns1")
	require.NoError(t, err)

	p, err := task.createSec(pattern, "www.example.com")
	require.NoError(t, err)
	assert.Equal(t, "ns1/sec/gw-www.example.com", p.String())
	_, err = task.createSec(pattern, "www.example.com")
	assert.ErrorContains(t, err, "is not a sec this task created")

	other := naming.Path{Namespace: "ns1", Kind: naming.KindSec, Name: "gw-hand.example.com"}
	require.NoError(t, os.MkdirAll(filepath.Dir(other.ConfigFile()), 0o755))
	require.NoError(t, os.WriteFile(other.ConfigFile(), []byte("[DEFAULT]\ncn = hand.example.com\n"), 0o600))

	owned := task.ownedSecs(pattern)
	assert.Equal(t, map[string]naming.Path{"www.example.com": p}, owned)

	store, err := object.NewDataStore(p)
	require.NoError(t, err)
	assert.Equal(t, "letsencrypt-staging", store.Config().GetString(keyOf("acme.directory")))
	assert.Equal(t, "admin@example.com", store.Config().GetString(keyOf("email")))

	task.Directory = "letsencrypt"
	task.Email = ""
	require.NoError(t, task.align(store, "www.example.com", true))
	store, err = object.NewDataStore(p)
	require.NoError(t, err)
	assert.Equal(t, "letsencrypt", store.Config().GetString(keyOf("acme.directory")))
	assert.Equal(t, "", store.Config().GetString(keyOf("email")))
}

func keyOf(s string) key.T {
	return key.Parse(s)
}
