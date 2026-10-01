package configkeywords

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/hostname"

	_ "github.com/opensvc/om3/v3/drivers/resappsimple"
	_ "github.com/opensvc/om3/v3/drivers/resfsflag"
	_ "github.com/opensvc/om3/v3/drivers/ressyncrsync"
)

func writeConfig(t *testing.T, p string, s string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0755))
	require.NoError(t, os.WriteFile(p, []byte(s), 0644))
}

func byKeyword(items api.KeywordItems) map[string]api.KeywordItem {
	m := make(map[string]api.KeywordItem)
	for _, item := range items {
		m[item.Keyword] = item
	}
	return m
}

func text(item api.KeywordItem) string {
	if item.EvaluatedText == nil {
		return "<nil>"
	}
	return *item.EvaluatedText
}

// The keywords of an object are answered with their raw value, and with
// their evaluated value and its text when asked: a keyword set nowhere is
// answered with its default.
func TestObject(t *testing.T) {
	testhelper.Setup(t)
	p, err := naming.ParsePath("test/vol/kw")
	require.NoError(t, err)
	writeConfig(t, p.ConfigFile(), "[DEFAULT]\nnodes = *\nsize = $(10g / 2)\n\n[env]\nfoo = bar\n")

	items, err := Object(p, Options{Keywords: []string{"DEFAULT.size", "env.foo"}, Evaluate: true})
	require.NoError(t, err)
	m := byKeyword(items)
	require.Len(t, m, 2)

	assert.Equal(t, "$(10g / 2)", m["size"].Value)
	assert.Equal(t, "5g", text(m["size"]))
	assert.Equal(t, hostname.Hostname(), m["size"].EvaluatedAs)
	assert.Equal(t, "bar", text(m["env.foo"]))
	assert.Equal(t, p.String(), m["env.foo"].Object)

	// A keyword set nowhere is answered with its default.
	svc, err := naming.ParsePath("test/svc/kw")
	require.NoError(t, err)
	writeConfig(t, svc.ConfigFile(), "[DEFAULT]\nnodes = *\n")
	items, err = Object(svc, Options{Keywords: []string{"DEFAULT.orchestrate"}, Evaluate: true})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "", items[0].Value, "set nowhere")
	assert.Equal(t, "no", text(items[0]), "the default")

	// Raw only.
	items, err = Object(p, Options{Keywords: []string{"env.foo"}})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "bar", items[0].Value)
	assert.Nil(t, items[0].Evaluated)

	// Impersonated.
	items, err = Object(p, Options{Keywords: []string{"env.foo"}, Evaluate: true, Impersonate: "n2"})
	require.NoError(t, err)
	assert.Equal(t, "n2", items[0].EvaluatedAs)
}

// A keyword a selection names and no keyword declares is an error. The whole
// configuration is answered all the same, with the error on the item.
func TestObjectUnknownKeyword(t *testing.T) {
	testhelper.Setup(t)
	p, err := naming.ParsePath("test/svc/kw")
	require.NoError(t, err)
	writeConfig(t, p.ConfigFile(), "[DEFAULT]\nnodes = *\nbogus = 1\n\n[env]\nfoo = bar\n")

	_, err = Object(p, Options{Keywords: []string{"DEFAULT.bogus"}, Evaluate: true})
	require.True(t, errors.Is(err, xconfig.ErrNoKeyword), "%v", err)

	items, err := Object(p, Options{Evaluate: true})
	require.NoError(t, err)
	m := byKeyword(items)
	// The keys of the DEFAULT section are listed without it.
	require.Contains(t, m, "bogus")
	require.NotNil(t, m["bogus"].Error)
	assert.Equal(t, "bar", text(m["env.foo"]))
}

// A secret is not shown to a reader not allowed to see it, raw or evaluated,
// and is listed all the same.
func TestObjectRedact(t *testing.T) {
	testhelper.Setup(t)
	// A sec object is read with the key of the cluster, which om loads
	// with the cluster configuration.
	clusterConfig := &cluster.Config{Name: "cluster1"}
	clusterConfig.SetSecret("0123456789abcdef0123456789abcdef")
	cluster.ConfigData.Set(clusterConfig)
	t.Cleanup(cluster.InitData)
	p, err := naming.ParsePath("test/sec/kw")
	require.NoError(t, err)
	writeConfig(t, p.ConfigFile(), "[DEFAULT]\n\n[data]\nkey1 = literal:secret\n")

	items, err := Object(p, Options{Keywords: []string{"data.key1"}, Evaluate: true, Redact: true})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, object.RedactedValue, items[0].Value)
	assert.Equal(t, object.RedactedValue, text(items[0]))
	assert.NotContains(t, (*items[0].Evaluated).(string), "secret")
}

// The keywords of the node are the ones that apply to it, from its
// configuration or the cluster's, and a keyword set nowhere evaluates to its
// default. The whole configuration is the keys the node configuration sets.
func TestNode(t *testing.T) {
	testhelper.Setup(t)
	writeConfig(t, rawconfig.NodeConfigFile(), "[node]\nmax_parallel = 7\n")
	writeConfig(t, rawconfig.ClusterConfigFile(), "[cluster]\nname = c1\n\n[node]\nenv = REC\n")

	items, err := Node("n1", Options{Keywords: []string{"node.max_parallel", "node.env", "node.rejoin_grace_period"}, Evaluate: true})
	require.NoError(t, err)
	m := byKeyword(items)
	assert.Equal(t, "7", text(m["node.max_parallel"]))
	assert.Equal(t, "REC", m["node.env"].Value, "set in the cluster configuration only")
	assert.Equal(t, "REC", text(m["node.env"]))
	assert.Equal(t, "", m["node.rejoin_grace_period"].Value, "set nowhere")
	assert.NotEqual(t, "<nil>", text(m["node.rejoin_grace_period"]), "evaluated to its default")
	assert.Equal(t, "n1", m["node.env"].Node)

	items, err = Node("n1", Options{})
	require.NoError(t, err)
	m = byKeyword(items)
	assert.Contains(t, m, "node.max_parallel")
	assert.NotContains(t, m, "node.env", "the whole configuration is the node's own")

	_, err = Node("n1", Options{Keywords: []string{"node.bogus"}, Evaluate: true})
	require.True(t, errors.Is(err, xconfig.ErrNoKeyword), "%v", err)
}

// The section of a keyword is a resource selector element when it is a driver
// group or a pattern, and the option is a pattern when it holds one. A
// keyword naming one key is not a pattern.
func TestIsPattern(t *testing.T) {
	for s, want := range map[string]bool{
		"container#1.stop_timeout": false,
		"DEFAULT.nodes":            false,
		"env.foo":                  false,
		"nodes":                    false,
		"container.stop_timeout":   true,
		"cont*.stop_timeout":       true,
		"*.stop_timeout":           true,
		"container#1.*":            true,
		"container#1.stop_*":       true,
		"hb#*.timeout":             true,
	} {
		assert.Equal(t, want, IsPattern(s), s)
	}
	assert.True(t, HasPattern([]string{"DEFAULT.nodes", "app.stop_timeout"}))
	assert.False(t, HasPattern([]string{"DEFAULT.nodes", "app#1.stop_timeout"}))
}

// A driver group or a pattern in the section answers the keyword of every
// resource it matches, set or not, and skips the resources whose driver does
// not have it. A pattern in the option answers every keyword of the section
// it matches, set or not.
func TestObjectPattern(t *testing.T) {
	testhelper.Setup(t)
	p, err := naming.ParsePath("test/svc/kw")
	require.NoError(t, err)
	writeConfig(t, p.ConfigFile(), `[DEFAULT]
nodes = *

[app#1]
type = simple
start = /bin/true
stop_timeout = 5s

[app#2]
type = simple
start = /bin/true

[fs#1]
type = flag

[sync#1]
type = rsync
src = fs#1
dst = /tmp/x

[env]
foo = bar
fob = baz
`)

	keywords := func(items api.KeywordItems) []string {
		l := make([]string, len(items))
		for i, item := range items {
			l[i] = item.Keyword
		}
		return l
	}

	t.Run("a driver group", func(t *testing.T) {
		items, err := Object(p, Options{Keywords: []string{"app.stop_timeout"}, Evaluate: true})
		require.NoError(t, err)
		assert.Equal(t, []string{"app#1.stop_timeout", "app#2.stop_timeout"}, keywords(items))
		m := byKeyword(items)
		assert.Equal(t, "5s", text(m["app#1.stop_timeout"]))
		assert.Equal(t, "", m["app#2.stop_timeout"].Value, "set nowhere")
		assert.NotEqual(t, "<nil>", text(m["app#2.stop_timeout"]), "evaluated to its default")
	})

	t.Run("a pattern skips the resources without the keyword", func(t *testing.T) {
		items, err := Object(p, Options{Keywords: []string{"*.src"}, Evaluate: true})
		require.NoError(t, err)
		assert.Equal(t, []string{"sync#1.src"}, keywords(items))
	})

	t.Run("a pattern matching nothing", func(t *testing.T) {
		items, err := Object(p, Options{Keywords: []string{"container.stop_timeout"}, Evaluate: true})
		require.NoError(t, err)
		assert.Empty(t, items)
	})

	t.Run("a pattern does not match the sections that are no resource", func(t *testing.T) {
		items, err := Object(p, Options{Keywords: []string{"*.foo"}})
		require.NoError(t, err)
		assert.Empty(t, items)
	})

	t.Run("an option pattern answers the keywords set or not", func(t *testing.T) {
		items, err := Object(p, Options{Keywords: []string{"app#2.stop*"}, Evaluate: true})
		require.NoError(t, err)
		m := byKeyword(items)
		assert.Contains(t, m, "app#2.stop_timeout")
		assert.Contains(t, m, "app#2.stop")
		assert.NotContains(t, m, "app#2.start", "not matched")
		for k := range m {
			assert.Contains(t, k, "app#2.stop", "only the section asked")
		}
	})

	t.Run("an option pattern in a section holding any option", func(t *testing.T) {
		items, err := Object(p, Options{Keywords: []string{"env.fo*"}, Evaluate: true})
		require.NoError(t, err)
		assert.Equal(t, []string{"env.fob", "env.foo"}, keywords(items))
	})

	t.Run("a key matched twice is answered once", func(t *testing.T) {
		items, err := Object(p, Options{Keywords: []string{"app#1.stop_timeout", "app.stop_timeout"}})
		require.NoError(t, err)
		assert.Equal(t, []string{"app#1.stop_timeout", "app#2.stop_timeout"}, keywords(items))
	})

	t.Run("a key named is still selected", func(t *testing.T) {
		_, err := Object(p, Options{Keywords: []string{"app#1.bogus"}, Evaluate: true})
		require.True(t, errors.Is(err, xconfig.ErrNoKeyword), "%v", err)
	})
}

// A pattern matches the sections of the node and of the cluster
// configurations, as a heartbeat is configured in the cluster's.
func TestNodePattern(t *testing.T) {
	testhelper.Setup(t)
	writeConfig(t, rawconfig.NodeConfigFile(), "[node]\nmax_parallel = 7\n")
	writeConfig(t, rawconfig.ClusterConfigFile(), "[cluster]\nname = c1\n\n[hb#1]\ntype = unicast\ntimeout = 20s\n\n[hb#2]\ntype = multicast\n")

	items, err := Node("n1", Options{Keywords: []string{"hb#*.timeout"}, Evaluate: true})
	require.NoError(t, err)
	m := byKeyword(items)
	require.Len(t, m, 2)
	assert.Equal(t, "20s", text(m["hb#1.timeout"]))
	assert.NotEqual(t, "<nil>", text(m["hb#2.timeout"]), "evaluated to its default")

	items, err = Node("n1", Options{Keywords: []string{"node.max_par*"}, Evaluate: true})
	require.NoError(t, err)
	m = byKeyword(items)
	assert.Equal(t, "7", text(m["node.max_parallel"]))
}
