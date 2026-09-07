package rescontainer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSearchDomains pins the list a container looks a short name up in: its
// own domain first, then each parent of it, so "pod2" reaches
// "pod2.root.svc.cluster1" from an object of the root namespace.
func TestSearchDomains(t *testing.T) {
	for _, tc := range []struct {
		name   string
		domain string
		extra  []string
		want   []string
	}{
		{
			name:   "an object domain walks up to the cluster",
			domain: "root.svc.cluster1",
			want:   []string{"root.svc.cluster1", "svc.cluster1", "cluster1"},
		},
		{
			name:   "another namespace searches its own first",
			domain: "test.vol.cluster1",
			want:   []string{"test.vol.cluster1", "vol.cluster1", "cluster1"},
		},
		{
			name:   "a configured domain comes first",
			domain: "root.svc.cluster1",
			extra:  []string{"corp.example"},
			want:   []string{"corp.example", "root.svc.cluster1", "svc.cluster1", "cluster1"},
		},
		{
			name:   "no object domain leaves the configured ones",
			domain: "",
			extra:  []string{"corp.example"},
			want:   []string{"corp.example"},
		},
		{
			name:   "a domain with no parent is the whole list",
			domain: "cluster1",
			want:   []string{"cluster1"},
		},
		{
			name:   "nothing to search",
			domain: "",
			want:   []string{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, SearchDomains(tc.domain, tc.extra))
		})
	}
}

func TestResolvConfString(t *testing.T) {
	resolvConf := ResolvConf{
		Nameservers: []string{"10.29.0.11", "10.29.0.12"},
		Searches:    []string{"root.svc.cluster1", "svc.cluster1"},
		Options:     ResolvConfOptions,
	}
	assert.Equal(t, `search root.svc.cluster1 svc.cluster1
nameserver 10.29.0.11
nameserver 10.29.0.12
options ndots:2 edns0 use-vc
`, resolvConf.String())
}

// TestResolvConfStopsAtTheNameserversAResolverReads pins that the file holds
// no line a resolver would not read. glibc stops at MAXNS, and a cluster can
// name more servers than that.
func TestResolvConfStopsAtTheNameserversAResolverReads(t *testing.T) {
	resolvConf := ResolvConf{
		Nameservers: []string{"10.29.0.11", "10.29.0.12", "10.29.0.13", "1.2.2.4"},
	}
	assert.Equal(t, `nameserver 10.29.0.11
nameserver 10.29.0.12
nameserver 10.29.0.13
`, resolvConf.String())
}

// TestResolvConfIsZero pins that a container is left with the resolver of its
// image rather than an empty file, when the cluster names none.
func TestResolvConfIsZero(t *testing.T) {
	assert.True(t, ResolvConf{Options: ResolvConfOptions}.IsZero())
	assert.False(t, ResolvConf{Nameservers: []string{"10.29.0.11"}}.IsZero())
	assert.False(t, ResolvConf{Searches: []string{"svc.cluster1"}}.IsZero())
}

func TestWriteResolvConfCreatesItsDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "container#1", "resolv.conf")
	got, err := WriteResolvConf(path, ResolvConf{Nameservers: []string{"10.29.0.11"}})
	require.NoError(t, err)
	assert.Equal(t, path, got)
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "nameserver 10.29.0.11\n", string(b))
}

// TestWriteResolvConfRewritesInPlace pins that a rewrite keeps the inode.
//
// A container bind mounts this file, and a bind mount follows the inode: a
// write to a new file renamed over this one would leave every running
// container reading the file it was started with, with nothing to say so.
func TestWriteResolvConfRewritesInPlace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "resolv.conf")
	_, err := WriteResolvConf(path, ResolvConf{Nameservers: []string{"10.29.0.11"}})
	require.NoError(t, err)
	before, err := os.Stat(path)
	require.NoError(t, err)

	_, err = WriteResolvConf(path, ResolvConf{Nameservers: []string{"10.29.0.99"}})
	require.NoError(t, err)
	after, err := os.Stat(path)
	require.NoError(t, err)

	assert.True(t, os.SameFile(before, after), "the rewrite must keep the inode a mount follows")
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "nameserver 10.29.0.99\n", string(b))
}
