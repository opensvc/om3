package daemonapi

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/objectdevice"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/hostname"
)

// The grants the instance endpoints of card 289 ask for follow the instance
// actions: a compliance fix, an auto run or an attachment changes the node,
// admin of the namespace as a provision; a check or a fixable test runs the
// modules, operator as an instance status; the env, the rules and the
// devices are reads, guest as the resource info; a key install refreshes
// what the instances install, operator as an instance install.
func TestInstanceEndpointsGrants(t *testing.T) {
	a := &DaemonAPI{localhost: "node1"}
	attach := true
	type call func(echo.Context) error
	calls := map[string]call{
		"fix": func(c echo.Context) error {
			return a.PostInstanceComplianceActionFix(c, "node2", "ns1", naming.KindSvc, "s1", api.PostInstanceComplianceActionFixParams{})
		},
		"auto": func(c echo.Context) error {
			return a.PostInstanceComplianceActionAuto(c, "node2", "ns1", naming.KindSvc, "s1", api.PostInstanceComplianceActionAutoParams{})
		},
		"check": func(c echo.Context) error {
			return a.PostInstanceComplianceActionCheck(c, "node2", "ns1", naming.KindSvc, "s1", api.PostInstanceComplianceActionCheckParams{})
		},
		"check attach": func(c echo.Context) error {
			return a.PostInstanceComplianceActionCheck(c, "node2", "ns1", naming.KindSvc, "s1", api.PostInstanceComplianceActionCheckParams{Attach: &attach})
		},
		"fixable": func(c echo.Context) error {
			return a.PostInstanceComplianceActionFixable(c, "node2", "ns1", naming.KindSvc, "s1", api.PostInstanceComplianceActionFixableParams{})
		},
		"env": func(c echo.Context) error {
			return a.GetInstanceComplianceEnv(c, "node2", "ns1", naming.KindSvc, "s1", api.GetInstanceComplianceEnvParams{})
		},
		"rules": func(c echo.Context) error {
			return a.GetInstanceComplianceRules(c, "node2", "ns1", naming.KindSvc, "s1")
		},
		"devices": func(c echo.Context) error {
			return a.GetInstanceDevices(c, "node2", "ns1", naming.KindSvc, "s1", api.GetInstanceDevicesParams{})
		},
		"key install": func(c echo.Context) error {
			return a.PostInstanceActionKeyInstall(c, "node2", "ns1", naming.KindSec, "s1", api.PostInstanceActionKeyInstallParams{})
		},
	}
	guest := rbac.Grants{rbac.NewGrant(rbac.RoleGuest, "ns1")}
	operator := rbac.Grants{rbac.NewGrant(rbac.RoleOperator, "ns1")}
	admin := rbac.Grants{rbac.NewGrant(rbac.RoleAdmin, "ns1")}
	other := rbac.Grants{rbac.NewGrant(rbac.RoleAdmin, "ns2")}
	for _, tc := range []struct {
		call    string
		allowed []rbac.Grants
		denied  []rbac.Grants
	}{
		{"fix", []rbac.Grants{admin}, []rbac.Grants{operator, guest, other}},
		{"auto", []rbac.Grants{admin}, []rbac.Grants{operator, guest, other}},
		{"check attach", []rbac.Grants{admin}, []rbac.Grants{operator, guest, other}},
		{"check", []rbac.Grants{admin, operator}, []rbac.Grants{guest, other}},
		{"fixable", []rbac.Grants{admin, operator}, []rbac.Grants{guest, other}},
		{"key install", []rbac.Grants{admin, operator}, []rbac.Grants{guest, other}},
		{"env", []rbac.Grants{admin, operator, guest}, []rbac.Grants{other}},
		{"rules", []rbac.Grants{admin, operator, guest}, []rbac.Grants{other}},
		{"devices", []rbac.Grants{admin, operator, guest}, []rbac.Grants{other}},
	} {
		t.Run(tc.call, func(t *testing.T) {
			for _, grants := range tc.denied {
				ctx, rec := newNodeTestContext(http.MethodPost, "/", nil, grants)
				require.NoError(t, calls[tc.call](ctx))
				assert.Equal(t, http.StatusForbidden, rec.Code, "grants %v", grants)
			}
			for _, grants := range tc.allowed {
				ctx, rec := newNodeTestContext(http.MethodPost, "/", nil, grants)
				// node2 is a peer the test daemon does not know:
				// past the grants, the proxy answers, and nothing
				// runs here.
				require.NoError(t, calls[tc.call](ctx))
				assert.NotEqual(t, http.StatusForbidden, rec.Code, "grants %v", grants)
			}
		})
	}
}

// Keys are installed from a datastore only.
func TestPostInstanceActionKeyInstallRefusesAnObjectHoldingNoKeys(t *testing.T) {
	a := &DaemonAPI{localhost: "node1"}
	ctx, rec := newNodeTestContext(http.MethodPost, "/", nil, rbac.Grants{rbac.GrantRoot})
	require.NoError(t, a.PostInstanceActionKeyInstall(ctx, "node1", "ns1", naming.KindSvc, "s1", api.PostInstanceActionKeyInstallParams{}))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// An object with no instance on the node is answered a 404, which a listing
// of several nodes takes as nothing to list there.
func TestInstanceEndpointsWithoutLocalInstance(t *testing.T) {
	setupNodeWithCollector(t)
	a := &DaemonAPI{localhost: hostname.Hostname()}
	ctx, rec := newNodeTestContext(http.MethodGet, "/", nil, rbac.Grants{rbac.GrantRoot})
	require.NoError(t, a.GetInstanceDevices(ctx, "localhost", "ns1", naming.KindSvc, "absent", api.GetInstanceDevicesParams{}))
	assert.Equal(t, http.StatusNotFound, rec.Code)

	ctx, rec = newNodeTestContext(http.MethodPost, "/", nil, rbac.Grants{rbac.GrantRoot})
	require.NoError(t, a.PostInstanceComplianceActionCheck(ctx, "localhost", "ns1", naming.KindSvc, "absent", api.PostInstanceComplianceActionCheckParams{}))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// The compliance of an object is asked of the collector for the object, and
// its run answered as a run of the node is.
func TestPostInstanceComplianceActionCheck(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("a compliance module is run only when owned by root")
	}
	fc := setupNodeWithCollector(t)
	p := naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "s1"}
	require.NoError(t, os.MkdirAll(filepath.Dir(p.ConfigFile()), 0o755))
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte("[DEFAULT]\nnodes = *\n"), 0o600))
	// The instance is on this node: the daemon published its configuration.
	instance.ConfigData.Set(p, hostname.Hostname(), &instance.Config{})
	t.Cleanup(func() { instance.ConfigData.Unset(p, hostname.Hostname()) })
	modDir := filepath.Join(rawconfig.Paths.Var, "compliance")
	require.NoError(t, os.MkdirAll(modDir, 0o755))
	script := "#!/bin/sh\necho checked $OSVC_COMP_MOTD\nexit 0\n"
	require.NoError(t, os.WriteFile(filepath.Join(modDir, "S10motd"), []byte(script), 0o755))

	a := &DaemonAPI{localhost: hostname.Hostname()}
	moduleset := "ms1"
	ctx, rec := newNodeTestContext(http.MethodPost, "/", nil, rbac.Grants{rbac.GrantRoot})
	require.NoError(t, a.PostInstanceComplianceActionCheck(ctx, "localhost", "root", naming.KindSvc, "s1", api.PostInstanceComplianceActionCheckParams{Moduleset: &moduleset}))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var run api.ComplianceRun
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &run))
	require.Len(t, run.ModuleActions, 1)
	assert.Equal(t, 0, run.ModuleActions[0].ExitCode)
	assert.Equal(t, []api.ComplianceLogEntry{{Level: api.ComplianceLogEntryLevelOut, Message: "checked hello"}}, run.ModuleActions[0].Log)
	assert.Contains(t, fc.requests(), "GET /api/agent/compliance/data")
	assert.Contains(t, fc.objectQueries(), "s1", "the collector is asked for the object")
}

func TestInstanceDeviceList(t *testing.T) {
	a := &DaemonAPI{localhost: "node1"}
	p := naming.Path{Namespace: "ns1", Kind: naming.KindSvc, Name: "s1"}
	l := objectdevice.L{{
		Device:     device.New("/dev/sda"),
		Role:       objectdevice.RoleExposed,
		RID:        "disk#1",
		DriverID:   driver.NewID(driver.GroupDisk, "disk"),
		ObjectPath: p,
	}}
	r := a.instanceDeviceList(l)
	assert.Equal(t, api.InstanceDeviceListKindInstanceDeviceList, r.Kind)
	require.Len(t, r.Items, 1)
	assert.Equal(t, api.InstanceMeta{Node: "node1", Object: "ns1/svc/s1"}, r.Items[0].Meta)
	assert.Equal(t, api.InstanceDevice{
		Path:   "ns1/svc/s1",
		Rid:    "disk#1",
		Driver: api.InstanceDeviceDriver{Group: "disk", Name: "disk"},
		Role:   api.InstanceDeviceRoleExposed,
		Device: "/dev/sda",
	}, r.Items[0].Data)
}

// A configuration file is no instance: a node out of the scope of the object
// keeps it. And an object of a kind with no compliance is not taken for a
// service of the same name.
func TestInstanceComplianceNeedsALocalInstanceOfItsKind(t *testing.T) {
	setupNodeWithCollector(t)
	p := naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "s2"}
	require.NoError(t, os.MkdirAll(filepath.Dir(p.ConfigFile()), 0o755))
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte("[DEFAULT]\nnodes = othernode\n"), 0o600))
	a := &DaemonAPI{localhost: hostname.Hostname()}
	ctx, rec := newNodeTestContext(http.MethodPost, "/", nil, rbac.Grants{rbac.GrantRoot})
	require.NoError(t, a.PostInstanceComplianceActionCheck(ctx, "localhost", "root", naming.KindSvc, "s2", api.PostInstanceComplianceActionCheckParams{}))
	assert.Equal(t, http.StatusNotFound, rec.Code)

	c := naming.Path{Namespace: "root", Kind: naming.KindCfg, Name: "s2"}
	require.NoError(t, os.MkdirAll(filepath.Dir(c.ConfigFile()), 0o755))
	require.NoError(t, os.WriteFile(c.ConfigFile(), []byte("[DEFAULT]\n"), 0o600))
	instance.ConfigData.Set(c, hostname.Hostname(), &instance.Config{})
	t.Cleanup(func() { instance.ConfigData.Unset(c, hostname.Hostname()) })
	ctx, rec = newNodeTestContext(http.MethodPost, "/", nil, rbac.Grants{rbac.GrantRoot})
	require.NoError(t, a.PostInstanceComplianceActionCheck(ctx, "localhost", "root", naming.KindCfg, "s2", api.PostInstanceComplianceActionCheckParams{}))
	assert.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
}
