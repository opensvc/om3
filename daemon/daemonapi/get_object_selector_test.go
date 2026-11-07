package daemonapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/plog"
)

// The paths a selector matches are a list, as every list the api answers
// with, {kind, items}: an empty one when nothing matches, not null.
func TestGetObjectPathsAnswersAList(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/object/path?path=nosuch877", nil)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.Set("logger", plog.NewDefaultLogger())
	c.Set("grants", rbac.Grants{rbac.GrantRoot})
	require.NoError(t, (&DaemonAPI{}).GetObjectPaths(c, api.GetObjectPathsParams{Path: "nosuch877"}))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"kind": "ObjectPathList", "items": []}`, rec.Body.String())
}
