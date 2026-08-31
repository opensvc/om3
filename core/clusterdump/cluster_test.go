package clusterdump

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/xmap"
)

func TestStatusUnmarshalJSON(t *testing.T) {
	var clusterStatus Data
	path := filepath.Join("test-fixtures", "clusterStatus.json")
	b, err := os.ReadFile(path)
	assert.Nil(t, err)
	err = json.Unmarshal(b, &clusterStatus)
	assert.Nil(t, err)
}

// TestFiltersDoNotModifyTheReceiver asserts the invariant the api handler
// relies on: ClusterData is behind a singleflight, so concurrent callers share
// the dataset pointer and a filter must not purge what it is given.
func TestFiltersDoNotModifyTheReceiver(t *testing.T) {
	newData := func() *Data {
		data := NewData("n1")
		nodeData := node.Node{Instance: map[string]instance.Instance{}}
		for _, ps := range []string{"ns1/svc/a", "ns2/svc/b"} {
			data.Cluster.Object[ps] = object.Status{}
			nodeData.Instance[ps] = instance.Instance{}
		}
		data.Cluster.Node["n1"] = nodeData
		return data
	}
	paths := func(data *Data) []string {
		l := append(xmap.Keys(data.Cluster.Object), xmap.Keys(data.Cluster.Node["n1"].Instance)...)
		sort.Strings(l)
		return l
	}
	all := []string{"ns1/svc/a", "ns1/svc/a", "ns2/svc/b", "ns2/svc/b"}
	ns1 := []string{"ns1/svc/a", "ns1/svc/a"}

	t.Run("WithNamespace", func(t *testing.T) {
		data := newData()
		assert.Equal(t, ns1, paths(data.WithNamespace("ns1")))
		assert.Equal(t, all, paths(data))
	})

	t.Run("WithSelector", func(t *testing.T) {
		data := newData()
		assert.Equal(t, ns1, paths(data.WithSelector("ns1/svc/*")))
		assert.Equal(t, all, paths(data))
	})

	t.Run("chained", func(t *testing.T) {
		data := newData()
		assert.Equal(t, ns1, paths(data.WithSelector("*/svc/*").WithNamespace("ns1")))
		assert.Equal(t, all, paths(data))
	})
}
