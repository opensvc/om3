package daemonapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/tombstone"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/plog"
)

// A create over a tombstone with pending holders is refused, saying why:
// a delete in progress on live holders, a node away on away holders.
func TestCreateRefusedOverATombstone(t *testing.T) {
	tombstone.InitData()
	t.Cleanup(tombstone.InitData)
	post := func(mode tombstone.HolderMode, dryRunForce bool) *httptest.ResponseRecorder {
		tombstone.Data.Set("n1", tombstone.Record{OrchestrationID: "a", Path: "tombtest/svc/c1", Kind: tombstone.KindPurged,
			Holders: map[string]tombstone.Holder{"n3": {State: tombstone.HolderPending, Mode: mode}}})
		target := "/"
		if dryRunForce {
			target = "/?force=true&dry_run=true"
		}
		req := httptest.NewRequest(http.MethodPost, target, nil)
		rec := httptest.NewRecorder()
		c := echo.New().NewContext(req, rec)
		c.Set("logger", plog.NewDefaultLogger())
		c.Set("grants", rbac.Grants{rbac.GrantRoot})
		params := api.PostObjectConfigFileParams{}
		if dryRunForce {
			v := true
			params.Force, params.DryRun = &v, &v
		}
		require.NoError(t, (&DaemonAPI{localhost: "n1"}).PostObjectConfigFile(c, "tombtest", "svc", "c1", params))
		return rec
	}
	rec := post(tombstone.HolderLive, false)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "being deleted on n3")
	rec = post(tombstone.HolderAway, false)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "n3 away")
	rec = post(tombstone.HolderAway, true)
	assert.NotContains(t, rec.Body.String(), "tombstone", "forced, the tombstone check passes")
	assert.True(t, tombstone.Data.IsPending("tombtest/svc/c1", "n3"), "a dry run drops nothing")
}

// A create --force refused by a check drops no tombstone: the object would
// come back from the away holder its config would be fetched from again.
func TestCreateForceRefusedDropsNothing(t *testing.T) {
	tombstone.InitData()
	t.Cleanup(tombstone.InitData)
	tombstone.Data.Set("n1", tombstone.Record{OrchestrationID: "a", Path: "tombtest/svc/c1", Kind: tombstone.KindPurged,
		Holders: map[string]tombstone.Holder{"n3": {State: tombstone.HolderPending, Mode: tombstone.HolderAway}}})
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("[DEFAULT]\nnot a config"))
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.Set("logger", plog.NewDefaultLogger())
	c.Set("grants", rbac.Grants{rbac.GrantRoot})
	force := true
	require.NoError(t, (&DaemonAPI{localhost: "n1"}).PostObjectConfigFile(c, "tombtest", "svc", "c1", api.PostObjectConfigFileParams{Force: &force}))
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.True(t, tombstone.Data.IsPending("tombtest/svc/c1", "n3"), "nothing dropped")
}
