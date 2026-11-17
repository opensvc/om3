package commoncmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/daemon/api"
)

// A tombstone is a row per holder.
func TestClusterTombstoneViews(t *testing.T) {
	kept := "kept"
	views := clusterTombstoneViews([]api.Tombstone{{
		OrchestrationID: "a", Path: "ns1/svc/s1", Kind: "purged",
		Holders: []api.TombstoneHolder{
			{Node: "n2", State: "pending", Mode: "live"},
			{Node: "n3", State: "cleared", Mode: "away", ClearedBy: &kept},
		},
	}})
	assert.Len(t, views, 2)
	assert.Equal(t, "n2", views[0].Holder)
	assert.Equal(t, "", views[0].ClearedBy)
	assert.Equal(t, "kept", views[1].ClearedBy)
	assert.Equal(t, "ns1/svc/s1", views[1].Path)
}
