package datarecv

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/hostname"
)

// The daemon refreshes the keys of a datastore in the local volumes when
// the datastores the configuration of an instance uses, by
// SectionDatastores, name it. The install command reads the lines with
// their own parser, HasMetadata: a datastore one of them names and the
// other does not is either refreshed for nothing, or not refreshed, so
// the two must agree.
func TestSectionDatastoresAgreesWithHasMetadata(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)

	p := naming.Path{Name: "gw", Kind: naming.KindSvc, Namespace: "ns1"}
	require.NoError(t, os.MkdirAll(filepath.Dir(p.ConfigFile()), 0o755))
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte("[DEFAULT]\nnodes = "+hostname.Hostname()+"\n"), 0o600))
	o, err := object.NewCore(p)
	require.NoError(t, err)

	install := []string{
		"/etc/", "mode", "0750",
		"/etc/haproxy.cfg", "from", "./cfg/haproxy", "key", "haproxy.cfg",
		"/etc/ssl/certs/*.pem", "from", "./sec/web-*", "key", "fullpem",
		"/etc/ca.pem", "from", "other/sec/ca", "key", "ca.pem",
		"/etc/all/", "from", "./cfg/all",
		"/etc/abs.conf", "from", "ns1/cfg/abs", "key", "abs.conf", "mode", "0600",
		"/web/index.html", "required", "source", "https://example.com/index.html",
	}
	configs := []string{"d/*:/cfg", "e/k:/e"}
	secrets := []string{"d/*:/sec", "u/k:/u"}
	recv := &DataRecv{
		to:      headReceiver{o: o, head: t.TempDir()},
		Install: install,
		Configs: configs,
		Secrets: secrets,
	}
	values := map[string][]string{"install": install, "configs": configs, "secrets": secrets}
	uses := SectionDatastores(func(option string) []string { return values[option] }, p.Namespace)
	usesPath := func(s naming.Path) bool {
		for _, use := range uses {
			if !naming.IsPathPattern(use) {
				if use == s.String() {
					return true
				}
				continue
			}
			if pattern, err := naming.ParsePathPatternRel(use, naming.NsRoot); err == nil && pattern.MatchPath(s) {
				return true
			}
		}
		return false
	}

	var used []string
	for _, s := range []string{
		"ns1/cfg/haproxy", "ns1/cfg/all", "ns1/cfg/abs", "ns1/cfg/d", "ns1/cfg/e",
		"ns1/sec/web-www", "ns1/sec/web-api", "ns1/sec/d", "ns1/sec/u",
		"other/sec/ca",
		"ns1/cfg/other", "ns1/sec/web", "ns1/sec/haproxy", "other/cfg/haproxy", "ns2/sec/web-www", "ns1/sec/ca", "ns1/cfg/u",
	} {
		store, err := naming.ParsePath(s)
		require.NoError(t, err)
		has := recv.HasMetadata(store, "")
		assert.Equal(t, has, usesPath(store), "%s: HasMetadata says %v", s, has)
		if has {
			used = append(used, s)
		}
	}
	slices.Sort(used)
	assert.Equal(t, []string{
		"ns1/cfg/abs", "ns1/cfg/all", "ns1/cfg/d", "ns1/cfg/e", "ns1/cfg/haproxy",
		"ns1/sec/d", "ns1/sec/u", "ns1/sec/web-api", "ns1/sec/web-www",
		"other/sec/ca",
	}, used, "the datastores the lines name")
}

// A template of a resource reading a key of a datastore at its last render
// is said by the file the render records, the daemon reading it to refresh
// the keys of the datastore.
func TestTemplateReadsFrom(t *testing.T) {
	testhelper.Setup(t)
	obj := naming.Path{Name: "gw", Kind: naming.KindSvc, Namespace: "ns1"}
	read := naming.Path{Name: "tls", Kind: naming.KindSec, Namespace: "ns1"}
	other := naming.Path{Name: "db", Kind: naming.KindSec, Namespace: "ns1"}

	assert.False(t, TemplateReadsFrom(obj, "volume#1", read), "no render recorded")

	filename := templateKeysPath(obj, "volume#1")
	require.NoError(t, os.MkdirAll(filepath.Dir(filename), 0o755))
	require.NoError(t, os.WriteFile(filename, []byte(`{"ns1/cfg/haproxy:haproxy.cfg": [{"path": "ns1/sec/tls", "key": "fullpem"}]}`), 0o600))
	assert.True(t, TemplateReadsFrom(obj, "volume#1", read))
	assert.False(t, TemplateReadsFrom(obj, "volume#1", other))
	assert.False(t, TemplateReadsFrom(obj, "volume#2", read), "another resource")
}
