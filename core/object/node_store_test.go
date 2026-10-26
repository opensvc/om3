package object_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/testhelper"

	_ "github.com/opensvc/om3/v3/drivers/storetar"
)

// A store of a sec is named from <sec> store#<name>, a ./ path naming a sec
// of the namespace of the object using it, which the sec must share with,
// and is reached over the network. A bare name is a store of the cluster
// configuration, never one of a sec.
func TestStoreOfASec(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)

	stores := naming.Path{Namespace: "backup", Kind: naming.KindSec, Name: "stores"}
	sec, err := object.NewSec(stores, object.WithConfigData([]byte(`[DEFAULT]
share = prd1

[store#remote]
type = tar
url = sftp://backup@sftp.example/srv/backups

[store#local]
type = tar
url = /srv/backups
`)))
	require.NoError(t, err)
	require.NoError(t, sec.Config().Commit())

	n, err := object.NewNode(object.WithVolatile(true))
	require.NoError(t, err)
	reader := func(ns string) naming.Path { return naming.Path{Namespace: ns, Kind: naming.KindSvc, Name: "web"} }

	s, err := n.Store("from ./sec/stores store#remote", reader("backup"))
	require.NoError(t, err, "a ./ path of the namespace of the reader")
	assert.Equal(t, "from backup/sec/stores store#remote", s.Ref())

	_, err = n.Store("from backup/sec/stores store#remote", reader("prd1"))
	assert.NoError(t, err, "a namespace the sec shares with")

	// A namespace the sec does not share with learns nothing of it: the
	// same refusal whether the sec, or the store, exists or not, and no
	// word of the namespaces it shares with.
	refused := `store "from backup/sec/stores store#remote": not usable from the prd2 namespace`
	_, err = n.Store("from backup/sec/stores store#remote", reader("prd2"))
	assert.ErrorIs(t, err, object.ErrStoreNotUsable)
	assert.EqualError(t, err, refused)
	_, err = n.Store("from backup/sec/stores store#nope", reader("prd2"))
	assert.EqualError(t, err, `store "from backup/sec/stores store#nope": not usable from the prd2 namespace`, "a store the sec does not declare")
	_, err = n.Store("from backup/sec/nope store#remote", reader("prd2"))
	assert.EqualError(t, err, `store "from backup/sec/nope store#remote": not usable from the prd2 namespace`, "a sec that does not exist")
	_, err = n.Store("from ./sec/nope store#remote", reader("backup"))
	assert.ErrorContains(t, err, "does not exist", "the namespace of the sec is told")

	_, err = n.Store("from ./sec/stores store#local", reader("backup"))
	assert.ErrorContains(t, err, "reached over the network")

	_, err = n.Store("from ./sec/stores store#nope", reader("backup"))
	assert.ErrorContains(t, err, "no store#nope section")

	_, err = n.Store("remote", reader("backup"))
	assert.ErrorContains(t, err, "no store#remote section in the cluster configuration", "a bare name is a store of the cluster configuration")

	items := n.ListStores()
	refs := make([]string, len(items))
	for i, item := range items {
		refs[i] = item.Ref
	}
	assert.Contains(t, refs, "from backup/sec/stores store#remote")
}
