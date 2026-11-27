package objectaction

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/core/nodesinfo"
	"github.com/opensvc/om3/v3/daemon/api"
)

func keys[V any](m map[string]V) []string {
	l := make([]string, 0, len(m))
	for k := range m {
		l = append(l, k)
	}
	sort.Strings(l)
	return l
}

// A namespace configuration is on every node of the cluster, and the daemon
// lists it with no instance: an action with --node is asked to every node
// selected. It was asked to none, "no instance of ns1/ on n2" and exit 0,
// so the cap of a namespace was taken as applied on a peer it never reached.
func TestANamespaceConfigActionIsAskedToTheNodesSelected(t *testing.T) {
	cluster := nodeselector.WithNodesInfo(nodesinfo.M{"n1": {}, "n2": {}, "n3": {}})
	nscfg, err := naming.ParsePath("ns1/")
	require.NoError(t, err)
	require.Equal(t, naming.KindNscfg, nscfg.Kind)

	// The daemon lists a namespace configuration with no data at all.
	var item api.ObjectItem
	require.NoError(t, json.Unmarshal([]byte(`{"kind": "ObjectItem", "meta": {"object": "ns1/"}, "data": null}`), &item))
	core, err := item.Data.AsObjectCore()
	require.NoError(t, err)

	t.Run("one peer", func(t *testing.T) {
		nodenames, err := selectedNodes("n2", cluster)
		require.NoError(t, err)
		assert.Equal(t, []string{"n2"}, keys(selectInstances(nscfg, core, nodenames)))
	})

	t.Run("every node", func(t *testing.T) {
		nodenames, err := selectedNodes("*", cluster)
		require.NoError(t, err)
		assert.Equal(t, []string{"n1", "n2", "n3"}, keys(selectInstances(nscfg, core, nodenames)))
	})

	t.Run("a node the cluster does not know", func(t *testing.T) {
		_, err := selectedNodes("n9", cluster)
		assert.EqualError(t, err, "no node matching n9")
	})
}

// The action on an object of another kind is asked to its instances and to
// the nodes of its scope, among the nodes selected, and to no other: a node
// selected the object is not on is not asked, and the command says so.
func TestAnActionIsAskedToTheNodesOfTheObjectOnly(t *testing.T) {
	svc := naming.Path{Namespace: "ns1", Kind: naming.KindSvc, Name: "web"}
	core := api.ObjectCore{
		Instances: api.InstanceMap{"n1": api.Instance{}},
		Scope:     []string{"n1", "n2"},
	}
	all := map[string]any{"n1": nil, "n2": nil, "n3": nil}
	assert.Equal(t, []string{"n1", "n2"}, keys(selectInstances(svc, core, all)), "n2 is in the scope, with no instance heard of yet")
	assert.Equal(t, []string{"n1"}, keys(selectInstances(svc, core, map[string]any{"n1": nil})))
	assert.Empty(t, selectInstances(svc, core, map[string]any{"n3": nil}), "no instance on n3")
	assert.Empty(t, selectInstances(svc, api.ObjectCore{}, all), "an object with no instance and no scope")
}
