package daemonapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/discovered"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/plog"
)

// The paths a selector matches are a list, as every list the api answers
// with, {kind, items}: an empty one when nothing matches, not null.
func TestGetObjectPathsAnswersAList(t *testing.T) {
	settled(t)
	req := httptest.NewRequest(http.MethodGet, "/api/object/path?path=nosuch877", nil)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.Set("logger", plog.NewDefaultLogger())
	c.Set("grants", rbac.Grants{rbac.GrantRoot})
	require.NoError(t, (&DaemonAPI{}).GetObjectPaths(c, api.GetObjectPathsParams{Path: "nosuch877"}))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"kind": "ObjectPathList", "items": []}`, rec.Body.String())
}

// settled marks the initial discovery done for the test, as a daemon started
// a while ago.
func settled(t *testing.T) {
	t.Helper()
	discovered.SetSettled()
	t.Cleanup(discovered.Reset)
}

// The objects of a daemon starting are not discovered yet: the paths of a
// selector are refused with a 503, which a client on the node takes to read
// its local files, rather than an empty list it would take for "no such
// object" (card 308).
func TestGetObjectPathsWhileDiscovering(t *testing.T) {
	discovered.Reset()
	for _, sel := range []string{"cluster", "**"} {
		req := httptest.NewRequest(http.MethodGet, "/api/object/path?path="+sel, nil)
		rec := httptest.NewRecorder()
		c := echo.New().NewContext(req, rec)
		c.Set("logger", plog.NewDefaultLogger())
		c.Set("grants", rbac.Grants{rbac.GrantRoot})
		require.NoError(t, (&DaemonAPI{}).GetObjectPaths(c, api.GetObjectPathsParams{Path: sel}))
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Contains(t, rec.Body.String(), "not discovered yet")
	}
	var m Meta
	assert.ErrorIs(t, m.expandPath(), ErrDiscovering, "the instance, resource and object listings")
}
