package daemonapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/plog"
)

func newGrantedContext(grants ...rbac.Grant) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	rec := httptest.NewRecorder()
	ctx := e.NewContext(httptest.NewRequest(http.MethodPut, "/", strings.NewReader("v")), rec)
	ctx.Set("grants", rbac.Grants(grants))
	ctx.Set("logger", plog.NewDefaultLogger())
	return ctx, rec
}

// The secreader role reads the key values of the sec objects of its
// namespace, and nothing else: the absent object answers 404 once the grant
// is accepted, 403 when it is not.
func TestSecReader(t *testing.T) {
	secreader := rbac.NewGrant(rbac.RoleSecReader, "prod")
	guest := rbac.NewGrant(rbac.RoleGuest, "prod")
	a := &DaemonAPI{localhost: "node1"}
	cases := []struct {
		name   string
		grants []rbac.Grant
		call   func(ctx echo.Context) error
		code   int
	}{
		{"secreader reads a sec key of its namespace", []rbac.Grant{secreader}, func(ctx echo.Context) error {
			return a.GetObjectDataKey(ctx, "prod", naming.KindSec, "db", api.GetObjectDataKeyParams{Name: "password"})
		}, http.StatusNotFound},
		{"secreader reads sec keys of its namespace", []rbac.Grant{secreader}, func(ctx echo.Context) error {
			return a.GetObjectData(ctx, "prod", naming.KindSec, "db", api.GetObjectDataParams{})
		}, http.StatusNotFound},
		{"a cluster-wide secreader reads a sec key", []rbac.Grant{rbac.NewGrant(rbac.RoleSecReader, "")}, func(ctx echo.Context) error {
			return a.GetObjectDataKey(ctx, "prod", naming.KindSec, "db", api.GetObjectDataKeyParams{Name: "password"})
		}, http.StatusNotFound},
		{"admin still reads a sec key", []rbac.Grant{rbac.NewGrant(rbac.RoleAdmin, "prod")}, func(ctx echo.Context) error {
			return a.GetObjectDataKey(ctx, "prod", naming.KindSec, "db", api.GetObjectDataKeyParams{Name: "password"})
		}, http.StatusNotFound},
		{"secreader of another namespace", []rbac.Grant{rbac.NewGrant(rbac.RoleSecReader, "dev")}, func(ctx echo.Context) error {
			return a.GetObjectDataKey(ctx, "prod", naming.KindSec, "db", api.GetObjectDataKeyParams{Name: "password"})
		}, http.StatusForbidden},
		{"guest does not read a sec key", []rbac.Grant{guest}, func(ctx echo.Context) error {
			return a.GetObjectDataKey(ctx, "prod", naming.KindSec, "db", api.GetObjectDataKeyParams{Name: "password"})
		}, http.StatusForbidden},
		{"guest does not read sec keys", []rbac.Grant{guest}, func(ctx echo.Context) error {
			return a.GetObjectData(ctx, "prod", naming.KindSec, "db", api.GetObjectDataParams{})
		}, http.StatusForbidden},
		{"secreader does not read a usr key", []rbac.Grant{secreader}, func(ctx echo.Context) error {
			return a.GetObjectDataKey(ctx, "prod", naming.KindUsr, "u", api.GetObjectDataKeyParams{Name: "password"})
		}, http.StatusForbidden},
		{"secreader does not read a cfg key alone", []rbac.Grant{secreader}, func(ctx echo.Context) error {
			return a.GetObjectDataKey(ctx, "prod", naming.KindCfg, "db", api.GetObjectDataKeyParams{Name: "host"})
		}, http.StatusForbidden},
		{"secreader does not add a sec key", []rbac.Grant{secreader}, func(ctx echo.Context) error {
			return a.PostObjectDataKey(ctx, "prod", naming.KindSec, "db", api.PostObjectDataKeyParams{Name: "k"})
		}, http.StatusForbidden},
		{"secreader does not change a sec key", []rbac.Grant{secreader}, func(ctx echo.Context) error {
			return a.PutObjectDataKey(ctx, "prod", naming.KindSec, "db", api.PutObjectDataKeyParams{Name: "k"})
		}, http.StatusForbidden},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx, rec := newGrantedContext(c.grants...)
			require.NoError(t, c.call(ctx))
			assert.Equal(t, c.code, rec.Code, rec.Body.String())
		})
	}
}

func TestParseRoleSecReader(t *testing.T) {
	assert.Equal(t, rbac.RoleSecReader, rbac.ParseRole("secreader"))
}
