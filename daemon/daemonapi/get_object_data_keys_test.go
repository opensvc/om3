package daemonapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/plog"
)

// An object no node holds used to be answered with a 200 status and an empty
// body, which a client decodes as an error instead of learning the object is
// absent.
func TestGetObjectDataKeysOfAnAbsentObject(t *testing.T) {
	e := echo.New()
	rec := httptest.NewRecorder()
	ctx := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
	ctx.Set("grants", rbac.Grants{rbac.GrantRoot})
	ctx.Set("logger", plog.NewDefaultLogger())
	a := &DaemonAPI{localhost: "node1"}
	require.NoError(t, a.GetObjectDataKeys(ctx, "test", naming.KindSec, "absent", api.GetObjectDataKeysParams{}))
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "test/sec/absent not found")
}
