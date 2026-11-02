package datarecv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
