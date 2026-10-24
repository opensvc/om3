package daemonapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/daemon/api"
)

// A cfg, a sec, a usr or the cluster configuration has a status and no
// resource: the listing goes past it, where it used to dereference the
// object part of its configuration, which it has none of, and crash.
func TestAppendResourceItemsGoesPastAnInstanceWithoutResources(t *testing.T) {
	cfg := naming.Path{Namespace: "ns1", Kind: naming.KindCfg, Name: "c1"}
	svc := naming.Path{Namespace: "ns1", Kind: naming.KindSvc, Name: "s1"}
	instance.StatusData.Set(cfg, "node1", &instance.Status{Avail: status.NotApplicable})
	instance.StatusData.Set(svc, "node1", &instance.Status{Avail: status.Up, Resources: instance.ResourceStatuses{"fs#1": resource.Status{Status: status.Up}}})
	t.Cleanup(func() {
		instance.StatusData.Unset(cfg, "node1")
		instance.StatusData.Unset(svc, "node1")
	})

	items := make(api.ResourceItems, 0)
	assert.NotPanics(t, func() {
		items = appendResourceItems(items, instance.DataElement[instance.Config]{Path: cfg, Node: "node1", Value: &instance.Config{Path: cfg}}, nil)
	})
	assert.Empty(t, items)

	items = appendResourceItems(items, instance.DataElement[instance.Config]{Path: svc, Node: "node1", Value: &instance.Config{Path: svc, ActorConfig: &instance.ActorConfig{Resources: instance.ResourceConfigs{"fs#1": {}}}}}, nil)
	require.Len(t, items, 1)
	assert.Equal(t, "ns1/svc/s1", items[0].Meta.Object)
}
