package daemonapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/plog"
)

func TestGetInstanceEnforcesNamespaceGrant(t *testing.T) {
	path := naming.Path{Namespace: "lab", Kind: naming.KindSvc, Name: "app"}
	instance.InitData()
	t.Cleanup(instance.InitData)
	instance.ConfigData.Set(path, "node1", &instance.Config{})

	t.Run("matching namespace", func(t *testing.T) {
		ctx, rec := newInstanceRequestContext("guest:lab")
		err := (&DaemonAPI{}).GetInstance(ctx, "node1", "lab", naming.KindSvc, "app")

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("other namespace", func(t *testing.T) {
		ctx, rec := newInstanceRequestContext("guest:other")
		err := (&DaemonAPI{}).GetInstance(ctx, "node1", "lab", naming.KindSvc, "app")

		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, rec.Code)
		assertProblemResponse(t, rec, http.StatusForbidden, "Forbidden")
	})
}

func TestGetInstanceHTTPErrorContract(t *testing.T) {
	instance.InitData()
	t.Cleanup(instance.InitData)

	t.Run("invalid kind", func(t *testing.T) {
		ctx, rec := newInstanceRequestContext("root")
		err := (&DaemonAPI{}).GetInstance(ctx, "node1", "lab", naming.Kind("invalid"), "app")

		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assertProblemResponse(t, rec, http.StatusBadRequest, "Invalid parameter")
	})

	t.Run("missing instance", func(t *testing.T) {
		ctx, rec := newInstanceRequestContext("root")
		err := (&DaemonAPI{}).GetInstance(ctx, "node1", "lab", naming.KindSvc, "missing")

		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assertProblemResponse(t, rec, http.StatusNotFound, "Not found")
		assert.Contains(t, rec.Body.String(), "lab/svc/missing@node1")
	})
}

func TestGetInstancePreservesZeroDates(t *testing.T) {
	path := naming.Path{Namespace: "lab", Kind: naming.KindSvc, Name: "app"}
	known := time.Date(2026, time.September, 28, 18, 0, 0, 0, time.UTC)
	instance.InitData()
	t.Cleanup(instance.InitData)
	instance.ConfigData.Set(path, "node1", &instance.Config{Checksum: "abc"})
	instance.MonitorData.Set(path, "node1", &instance.Monitor{
		IsPreserved: true,
		Resources: instance.ResourceMonitors{
			"app#worker": {Restart: &instance.ResourceMonitorRestart{}},
		},
	})
	instance.StatusData.Set(path, "node1", &instance.Status{
		UpdatedAt: known,
		Encap: instance.EncapMap{
			"guest": {Hostname: "guest"},
		},
		Resources: instance.ResourceStatuses{
			"app#worker": {
				Files:         resource.Files{{Name: "state"}},
				Info:          map[string]any{"large": int64(9007199254740993)},
				IsProvisioned: resource.ProvisionStatus{},
			},
		},
		Running: resource.RunningInfoList{{RID: "app#worker"}},
	})

	ctx, rec := newInstanceRequestContext("guest:lab")
	err := (&DaemonAPI{}).GetInstance(ctx, "node1", "lab", naming.KindSvc, "app")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)

	var response struct {
		Data struct {
			Config struct {
				Checksum  string     `json:"csum"`
				UpdatedAt *time.Time `json:"updated_at"`
			} `json:"config"`
			Monitor struct {
				GlobalExpectUpdatedAt   *time.Time `json:"global_expect_updated_at"`
				LocalExpectUpdatedAt    *time.Time `json:"local_expect_updated_at"`
				StateUpdatedAt          *time.Time `json:"state_updated_at"`
				MonitorActionExecutedAt *time.Time `json:"monitor_action_executed_at"`
				UpdatedAt               *time.Time `json:"updated_at"`
				Preserved               bool       `json:"preserved"`
				Resources               map[string]struct {
					Restart struct {
						LastAt *time.Time `json:"last_at"`
					} `json:"restart"`
				} `json:"resources"`
			} `json:"monitor"`
			Status struct {
				FrozenAt      *time.Time `json:"frozen_at"`
				LastStartedAt *time.Time `json:"last_started_at"`
				StoppedAt     *time.Time `json:"stopped_at"`
				UpdatedAt     *time.Time `json:"updated_at"`
				Encap         map[string]struct {
					UpdatedAt *time.Time `json:"updated_at"`
				} `json:"encap"`
				Resources map[string]struct {
					Provisioned struct {
						Mtime *time.Time `json:"mtime"`
					} `json:"provisioned"`
					Files []struct {
						Mtime *time.Time `json:"mtime"`
					} `json:"files"`
				} `json:"resources"`
				Running []struct {
					At *time.Time `json:"at"`
				} `json:"running"`
			} `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))

	assert.Equal(t, "abc", response.Data.Config.Checksum)
	assert.True(t, response.Data.Monitor.Preserved)
	require.Len(t, response.Data.Status.Resources["app#worker"].Files, 1)
	require.Len(t, response.Data.Status.Running, 1)
	for field, date := range map[string]*time.Time{
		"config.updated_at":                  response.Data.Config.UpdatedAt,
		"monitor.global_expect_updated_at":   response.Data.Monitor.GlobalExpectUpdatedAt,
		"monitor.local_expect_updated_at":    response.Data.Monitor.LocalExpectUpdatedAt,
		"monitor.state_updated_at":           response.Data.Monitor.StateUpdatedAt,
		"monitor.monitor_action_executed_at": response.Data.Monitor.MonitorActionExecutedAt,
		"monitor.updated_at":                 response.Data.Monitor.UpdatedAt,
		"monitor.resources.restart.last_at":  response.Data.Monitor.Resources["app#worker"].Restart.LastAt,
		"status.frozen_at":                   response.Data.Status.FrozenAt,
		"status.last_started_at":             response.Data.Status.LastStartedAt,
		"status.stopped_at":                  response.Data.Status.StoppedAt,
		"status.encap.updated_at":            response.Data.Status.Encap["guest"].UpdatedAt,
		"status.resources.provisioned.mtime": response.Data.Status.Resources["app#worker"].Provisioned.Mtime,
		"status.resources.files.mtime":       response.Data.Status.Resources["app#worker"].Files[0].Mtime,
		"status.running.at":                  response.Data.Status.Running[0].At,
	} {
		t.Run(field, func(t *testing.T) {
			require.NotNil(t, date)
			assert.True(t, date.IsZero())
		})
	}
	require.NotNil(t, response.Data.Status.UpdatedAt)
	assert.Equal(t, known, *response.Data.Status.UpdatedAt)
	assert.Contains(t, rec.Body.String(), `"large":9007199254740993`)
	assert.NotContains(t, rec.Body.String(), `"checksum"`)
	assert.NotContains(t, rec.Body.String(), `"is_preserved"`)
}

func newInstanceRequestContext(grants ...string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.Set("grants", rbac.NewGrants(grants...))
	ctx.Set("logger", plog.NewDefaultLogger())
	return ctx, rec
}

func assertProblemResponse(t *testing.T, rec *httptest.ResponseRecorder, status int, title string) {
	t.Helper()
	var problem api.Problem
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &problem))
	assert.Equal(t, status, problem.Status)
	assert.Equal(t, title, problem.Title)
}
