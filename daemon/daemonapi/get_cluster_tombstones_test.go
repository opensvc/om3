package daemonapi

import (
	"encoding/json"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	instance.InitData()
	t.Cleanup(instance.InitData)
	version := time.Date(2026, 11, 17, 15, 33, 47, 0, time.UTC)
	tombstone.Data.Set("n1", tombstone.Record{OrchestrationID: "a", Path: "ns1/svc/s1", Kind: tombstone.KindPurged, ObjectID: "o1", Version: version}, true)
	tombstone.Data.Set("n2", tombstone.Record{OrchestrationID: "a", Path: "ns1/svc/s1", Kind: tombstone.KindPurged, Version: version}, true)
	tombstone.Data.Set("n1", tombstone.Record{OrchestrationID: "b", Path: "ns2/sec/s2", Kind: tombstone.KindDeleted, Version: version}, true)
	p := naming.Path{Namespace: "ns1", Kind: naming.KindSvc, Name: "s1"}
	instance.ConfigData.Set(p, "n3", &instance.Config{Path: p, UpdatedAt: version})
	instance.ConfigData.Set(p, "n4", &instance.Config{Path: p, UpdatedAt: version.Add(time.Second)})

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
	require.Len(t, l.Items, 3, "a record per node writing it")
	assert.Equal(t, "ns1/svc/s1", l.Items[0].Path)
	assert.Equal(t, "n1", l.Items[0].Node)
	assert.Equal(t, "n2", l.Items[1].Node)
	assert.Equal(t, []string{"n3"}, l.Items[0].Holders, "n3 publishes the version deleted, n4 a newer one")
	require.NotNil(t, l.Items[0].ObjectID)
	assert.Equal(t, "o1", *l.Items[0].ObjectID)
	assert.Empty(t, l.Items[2].Holders)

	l = list(rbac.Grants{rbac.NewGrant(rbac.RoleGuest, "ns2")})
	require.Len(t, l.Items, 1)
	assert.Equal(t, "ns2/sec/s2", l.Items[0].Path)
}
