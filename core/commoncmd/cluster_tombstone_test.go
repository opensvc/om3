package commoncmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/daemon/api"
)

// A tombstone is a row, its holders listed.
func TestClusterTombstoneViews(t *testing.T) {
	views := clusterTombstoneViews([]api.Tombstone{{
		OrchestrationID: "a", Node: "n1", Path: "ns1/svc/s1", Kind: "purged",
		Holders: []string{"n2", "n3"},
	}})
	assert.Len(t, views, 1)
	assert.Equal(t, "n1", views[0].Node)
	assert.Equal(t, "n2 n3", views[0].Holders)
	assert.Equal(t, "ns1/svc/s1", views[0].Path)
}
