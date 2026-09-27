package daemonapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/key"
)

// The cluster configuration holds the secret every sec and usr value is
// encrypted with: root and a joining node read it, no guest does. The other
// configurations are read by the guests of their namespace, with their
// secrets redacted unless the reader administers the namespace, is root, or
// joins the cluster.
func TestConfigReadAccess(t *testing.T) {
	ccfg := naming.Cluster
	sec, _ := naming.ParsePath("system/sec/ca")
	usr, _ := naming.ParsePath("system/usr/u1")
	svc, _ := naming.ParsePath("ns1/svc/s1")
	for _, tc := range []struct {
		name   string
		grants rbac.Grants
		p      naming.Path
		ok     bool
		redact bool
	}{
		{"guest of root on the cluster config", rbac.NewGrants("guest:root"), ccfg, false, false},
		{"admin of root on the cluster config", rbac.NewGrants("admin:root"), ccfg, false, false},
		{"root on the cluster config", rbac.Grants{rbac.GrantRoot}, ccfg, true, false},
		{"join on the cluster config", rbac.Grants{rbac.GrantJoin}, ccfg, true, false},
		{"guest of system on a sec", rbac.NewGrants("guest:system"), sec, true, true},
		{"guest of system on a usr", rbac.NewGrants("guest:system"), usr, true, true},
		{"admin of system on a sec", rbac.NewGrants("admin:system"), sec, true, false},
		{"join on a sec", rbac.Grants{rbac.GrantJoin}, sec, true, false},
		{"root on a usr", rbac.Grants{rbac.GrantRoot}, usr, true, false},
		{"guest of ns1 on a sec of system", rbac.NewGrants("guest:ns1"), sec, false, false},
		{"guest of ns1 on a svc of ns1", rbac.NewGrants("guest:ns1"), svc, true, true},
		{"admin of ns1 on a svc of ns1", rbac.NewGrants("admin:ns1"), svc, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			ctx := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
			ctx.Set("grants", tc.grants)
			redact, ok, _ := configReadAccess(ctx, tc.p)
			assert.Equal(t, tc.ok, ok, "allowed")
			if ok {
				assert.Equal(t, tc.redact, redact, "redacted")
			}
		})
	}
}

func TestIsSecretKey(t *testing.T) {
	assert.True(t, object.IsSecretKey(naming.KindSec, key.New("data", "private_key"), ""))
	assert.True(t, object.IsSecretKey(naming.KindUsr, key.New("data", "password"), ""))
	assert.False(t, object.IsSecretKey(naming.KindSec, key.New("DEFAULT", "id"), ""))
	assert.False(t, object.IsSecretKey(naming.KindCfg, key.New("data", "foo"), ""), "a cfg key is no secret")
}
