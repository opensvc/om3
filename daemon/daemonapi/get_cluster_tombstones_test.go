package daemonapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/tombstone"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/plog"
)

// The tombstones are listed merged, root seeing them all, another user the
// ones of the namespaces it is guest of.
func TestGetClusterTombstones(t *testing.T) {
	tombstone.InitData()
	t.Cleanup(tombstone.InitData)
	pending := tombstone.Holder{State: tombstone.HolderPending, Mode: tombstone.HolderAway}
	tombstone.Data.Set("n1", tombstone.Record{OrchestrationID: "a", Path: "ns1/svc/s1", Kind: tombstone.KindPurged,
		Holders: map[string]tombstone.Holder{"n3": pending}})
	tombstone.Data.Set("n2", tombstone.Record{OrchestrationID: "a", Path: "ns1/svc/s1", Kind: tombstone.KindPurged,
		Holders: map[string]tombstone.Holder{"n3": pending}})
	tombstone.Data.Set("n1", tombstone.Record{OrchestrationID: "b", Path: "ns2/sec/s2", Kind: tombstone.KindDeleted,
		Holders: map[string]tombstone.Holder{"n2": {State: tombstone.HolderCleared, Mode: tombstone.HolderLive, ClearedBy: tombstone.ClearedByDeleted}}})

	list := func(grants rbac.Grants) api.TombstoneList {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := echo.New().NewContext(req, rec)
		c.Set("logger", plog.NewDefaultLogger())
		c.Set("grants", grants)
		require.NoError(t, (&DaemonAPI{localhost: "n1"}).GetClusterTombstones(c))
		require.Equal(t, http.StatusOK, rec.Code)
		var l api.TombstoneList
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &l))
		return l
	}
	l := list(rbac.Grants{rbac.GrantRoot})
	require.Len(t, l.Items, 2)
	assert.Equal(t, "ns1/svc/s1", l.Items[0].Path)
	assert.Equal(t, []string{"n1", "n2"}, l.Items[0].Nodes)
	assert.Equal(t, api.TombstoneHolderMode("away"), l.Items[0].Holders[0].Mode)
	require.NotNil(t, l.Items[1].Holders[0].ClearedBy)
	assert.Equal(t, "deleted", *l.Items[1].Holders[0].ClearedBy)

	l = list(rbac.Grants{rbac.NewGrant(rbac.RoleGuest, "ns2")})
	require.Len(t, l.Items, 1)
	assert.Equal(t, "ns2/sec/s2", l.Items[0].Path)
}
