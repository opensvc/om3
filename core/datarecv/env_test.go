package datarecv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/testhelper"
)

func TestParseEnvironment(t *testing.T) {
	l, err := ParseEnvironment([]string{
		"SLIP=slob",
		"OC3_DB_PASSWORD", "from", "./sec/oc3", "key", "db_password",
		"HOME",
		"*", "from", "./cfg/oc3", "key", "app_*",
		"OC3_*", "from", "other/sec/oc3", "key", "*",
	}, "dev")
	require.NoError(t, err)
	require.Len(t, l, 5)
	assert.Equal(t, "SLIP=slob", l[0].Literal)
	assert.Equal(t, "OC3_DB_PASSWORD", l[1].Name)
	assert.Equal(t, naming.Path{Namespace: "dev", Kind: naming.KindSec, Name: "oc3"}, l[1].Ref.Path)
	assert.Equal(t, "db_password", l[1].Ref.Key)
	assert.Equal(t, "HOME", l[2].Literal, "a bare name, as an engine reads it")
	assert.Equal(t, "app_*", l[3].Ref.Key)
	assert.Equal(t, "other", l[4].Ref.Path.Namespace)

	for name, items := range map[string][]string{
		"a pattern seeded":            {"A_*", "from", "./sec/oc3", "key", "db_*", "source", "random"},
		"truncated":                   {"A", "from", "./sec/oc3"},
		"no key word":                 {"A", "from", "./sec/oc3", "name", "k"},
		"a svc":                       {"A", "from", "./svc/oc3", "key", "k"},
		"a pattern with no * in name": {"A", "from", "./sec/oc3", "key", "db_*"},
		"not a variable name":         {"A-B", "from", "./sec/oc3", "key", "k"},
		"a pattern name of a digit":   {"9*", "from", "./sec/oc3", "key", "db_*"},
	} {
		_, err := ParseEnvironment(items, "dev")
		assert.Error(t, err, name)
	}
}

// The variables of a reference are read from its datastore: a key, or the keys
// a pattern matches named after them, secret when read from a sec, and a
// datastore or a key not existing passed to missing.
func TestEnvironment(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)

	sec := naming.Path{Namespace: "dev", Kind: naming.KindSec, Name: "oc3"}
	cfg := naming.Path{Namespace: "dev", Kind: naming.KindCfg, Name: "oc3"}
	for p, keys := range map[naming.Path]map[string]string{
		sec: {"db_password": "s3cr3t", "db_user": "oc3"},
		cfg: {"bar": "foo", "app_a": "1", "app_b": "2"},
	} {
		ds, err := object.NewDataStore(p)
		require.NoError(t, err)
		for k, v := range keys {
			require.NoError(t, ds.AddKey(k, []byte(v)))
		}
	}

	var missing []string
	l, secrets, err := Environment([]string{
		"SLIP=slob",
		"OC3_DB_PASSWORD", "from", "./sec/oc3", "key", "db_password",
		"OC3_FOO", "from", "./cfg/oc3", "key", "bar",
		"*", "from", "./cfg/oc3", "key", "app_*",
		"DB_*", "from", "./sec/oc3", "key", "db_user",
		"NONE", "from", "./cfg/none", "key", "x",
		"NOKEY", "from", "./cfg/oc3", "key", "nokey",
	}, "dev", func(err error) error {
		missing = append(missing, err.Error())
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"SLIP=slob", "OC3_DB_PASSWORD=s3cr3t", "OC3_FOO=foo", "app_a=1", "app_b=2", "DB_db_user=oc3"}, l)
	assert.Equal(t, map[string]bool{"OC3_DB_PASSWORD": true, "DB_db_user": true}, secrets)
	assert.Len(t, missing, 2)

	_, _, err = Environment([]string{"NOKEY", "from", "./cfg/oc3", "key", "nokey"}, "dev", func(err error) error { return err })
	assert.ErrorAs(t, err, &ErrEnvSourceMissing{})
}

// A reference ends after its key, or after the source seeding the key: a bare
// name "source" right after a reference is its source, unless it ends the
// list or names the next reference.
func TestParseEnvironmentSource(t *testing.T) {
	l, err := ParseEnvironment([]string{
		"OC3_DB_PASSWORD", "from", "./sec/oc3", "key", "db_password", "source", "random",
		"SLIP=slob",
		"OC3_FOO", "from", "./cfg/oc3", "key", "bar", "source", "https://example.com/bar",
		"OC3_BAR", "from", "./cfg/oc3", "key", "baz",
	}, "dev")
	require.NoError(t, err)
	require.Len(t, l, 4)
	assert.Equal(t, "random", l[0].Ref.Source)
	assert.True(t, l[0].Ref.SourceIsRandom())
	assert.Equal(t, "OC3_DB_PASSWORD from dev/sec/oc3 key db_password source random", l[0].String())
	assert.Equal(t, "SLIP=slob", l[1].Literal)
	assert.Equal(t, "https://example.com/bar", l[2].Ref.Source)
	assert.Equal(t, "", l[3].Ref.Source)
	assert.Equal(t, "OC3_BAR from dev/cfg/oc3 key baz", l[3].String())

	l, err = ParseEnvironment([]string{"A", "from", "./sec/a", "key", "k", "source"}, "dev")
	require.NoError(t, err)
	require.Len(t, l, 2, "a source ending the list is a bare name")
	assert.Equal(t, "source", l[1].Literal)

	l, err = ParseEnvironment([]string{"A", "from", "./sec/a", "key", "k", "source", "from", "./sec/b", "key", "k2"}, "dev")
	require.NoError(t, err)
	require.Len(t, l, 2, "a source followed by from names the next reference")
	assert.Equal(t, "", l[0].Ref.Source)
	assert.Equal(t, "source", l[1].Name)

	ks, err := EnvironmentKeySources([]string{
		"A", "from", "./sec/a", "key", "k", "source", "random",
		"B", "from", "other/cfg/b", "key", "k", "source", "/etc/x",
		"C", "from", "./cfg/c", "key", "k",
	}, "dev")
	require.NoError(t, err)
	assert.Equal(t, []keywords.KeySource{
		{Path: naming.Path{Namespace: "dev", Kind: naming.KindSec, Name: "a"}, Key: "k", Source: "random"},
		{Path: naming.Path{Namespace: "other", Kind: naming.KindCfg, Name: "b"}, Key: "k", Source: "/etc/x"},
	}, ks)
}

// A key generated missing is an error where it is read, never a variable
// gone without or a password made on the spot: the provision of the leader
// generates it. A key of another source missing is passed to missing.
func TestEnvironmentRandomKeyMissing(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)

	ignore := func(error) error { return nil }
	for name, items := range map[string][]string{
		"no datastore": {"PW", "from", "./sec/none", "key", "pw", "source", "random"},
		"no key":       {"PW", "from", "./sec/oc3", "key", "pw", "source", "random"},
	} {
		if name == "no key" {
			ds, err := object.NewDataStore(naming.Path{Namespace: "dev", Kind: naming.KindSec, Name: "oc3"})
			require.NoError(t, err)
			require.NoError(t, ds.AddKey("other", []byte("x")))
		}
		_, _, err := Environment(items, "dev", ignore)
		var e object.ErrRandomKeyMissing
		require.ErrorAs(t, err, &e, name)
		assert.Equal(t, "pw", e.Key, name)
		assert.Contains(t, err.Error(), "pw", name)
	}

	l, _, err := Environment([]string{"PW", "from", "./sec/oc3", "key", "pw", "source", "https://example.com/pw"}, "dev", ignore)
	require.NoError(t, err, "a key of another source missing is the resource's to go without")
	assert.Empty(t, l)

	ds, err := object.NewDataStore(naming.Path{Namespace: "dev", Kind: naming.KindSec, Name: "oc3"})
	require.NoError(t, err)
	require.NoError(t, ds.AddKey("pw", []byte("s3cr3t")))
	l, _, err = Environment([]string{"PW", "from", "./sec/oc3", "key", "pw", "source", "random"}, "dev", ignore)
	require.NoError(t, err)
	assert.Equal(t, []string{"PW=s3cr3t"}, l)
}
