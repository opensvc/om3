package daemonapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/plog"
)

// The config of an object a purge or a delete removes is not served: a peer
// installing it would bring back an instance the orchestration deletes, as
// v2 refused to fetch a config then (qatests 308).
func TestTheConfigOfAnObjectBeingPurgedIsNotServed(t *testing.T) {
	p := naming.Path{Namespace: "c308", Kind: naming.KindSvc, Name: "purge"}
	instance.ConfigData.Set(p, "n1", &instance.Config{})
	instance.MonitorData.Set(p, "n1", &instance.Monitor{GlobalExpect: instance.MonitorGlobalExpectPurged})
	t.Cleanup(func() {
		instance.ConfigData.Unset(p, "n1")
		instance.MonitorData.Unset(p, "n1")
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.Set("logger", plog.NewDefaultLogger())
	c.Set("grants", rbac.Grants{rbac.GrantRoot})
	a := &DaemonAPI{localhost: "n1"}
	require.NoError(t, a.GetObjectConfigFile(c, p.Namespace, p.Kind, p.Name, api.GetObjectConfigFileParams{}))
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "being deleted")
}

// The instance config endpoint, the one discover fetches a peer config
// through, refuses the config of an object being purged too.
func TestTheInstanceConfigOfAnObjectBeingPurgedIsNotServed(t *testing.T) {
	p := naming.Path{Namespace: "c308", Kind: naming.KindSvc, Name: "purge2"}
	instance.MonitorData.Set(p, "n1", &instance.Monitor{GlobalExpect: instance.MonitorGlobalExpectDeleted})
	t.Cleanup(func() { instance.MonitorData.Unset(p, "n1") })
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.Set("logger", plog.NewDefaultLogger())
	c.Set("grants", rbac.Grants{rbac.GrantRoot})
	a := &DaemonAPI{localhost: "n1"}
	require.NoError(t, a.GetInstanceConfigFile(c, "n1", p.Namespace, p.Kind, p.Name))
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "being deleted")
}

// The config of an object the local monitor deletes is not served from a
// peer either, the local config record gone already.
func TestTheConfigOfAnObjectBeingDeletedIsNotProxied(t *testing.T) {
	p := naming.Path{Namespace: "c308", Kind: naming.KindSvc, Name: "purge3"}
	instance.MonitorData.Set(p, "n1", &instance.Monitor{GlobalExpect: instance.MonitorGlobalExpectPurged})
	instance.ConfigData.Set(p, "n2", &instance.Config{})
	t.Cleanup(func() {
		instance.MonitorData.Unset(p, "n1")
		instance.ConfigData.Unset(p, "n2")
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.Set("logger", plog.NewDefaultLogger())
	c.Set("grants", rbac.Grants{rbac.GrantRoot})
	a := &DaemonAPI{localhost: "n1"}
	require.NoError(t, a.GetObjectConfigFile(c, p.Namespace, p.Kind, p.Name, api.GetObjectConfigFileParams{}))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
