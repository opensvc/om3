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

func newAbsentObjectContext() (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	rec := httptest.NewRecorder()
	ctx := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
	ctx.Set("grants", rbac.Grants{rbac.GrantRoot})
	ctx.Set("logger", plog.NewDefaultLogger())
	return ctx, rec
}

// A key of an object no node holds used to be answered with a 200 status and
// an empty body: a client read an empty value instead of learning the object
// is absent.
func TestGetObjectDataKeyOfAnAbsentObject(t *testing.T) {
	ctx, rec := newAbsentObjectContext()
	a := &DaemonAPI{localhost: "node1"}
	require.NoError(t, a.GetObjectDataKey(ctx, "test", naming.KindCfg, "absent", api.GetObjectDataKeyParams{Name: "k"}))
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "object test/cfg/absent not found")
}

func TestGetObjectDataOfAnAbsentObject(t *testing.T) {
	ctx, rec := newAbsentObjectContext()
	a := &DaemonAPI{localhost: "node1"}
	require.NoError(t, a.GetObjectData(ctx, "test", naming.KindCfg, "absent", api.GetObjectDataParams{}))
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "object test/cfg/absent not found")
}
