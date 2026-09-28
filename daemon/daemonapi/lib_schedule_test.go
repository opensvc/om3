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

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/schedule"
	"github.com/opensvc/om3/v3/daemon/rbac"
)

func TestScheduleItemUsesNullForUnknownDates(t *testing.T) {
	item := scheduleItem(schedule.Entry{
		Config: schedule.Config{
			Action:   "status",
			Key:      "DEFAULT.status_schedule",
			Schedule: "@10m",
		},
		Node: "node1",
		Path: naming.Path{Namespace: "ns1", Kind: naming.KindSvc, Name: "app"},
	})

	b, err := json.Marshal(item)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"kind": "ScheduleItem",
		"meta": {"node": "node1", "object": "ns1/svc/app"},
		"data": {
			"action": "status",
			"key": "DEFAULT.status_schedule",
			"last_run_at": null,
			"max_parallel": 0,
			"next_run_at": null,
			"require": "",
			"require_collector": false,
			"require_provisioned": false,
			"schedule": "@10m"
		}
	}`, string(b))
}

func TestScheduleItemKeepsKnownDates(t *testing.T) {
	last := time.Date(2026, time.September, 28, 8, 0, 0, 0, time.UTC)
	next := last.Add(10 * time.Minute)
	item := scheduleItem(schedule.Entry{LastRunAt: last, NextRunAt: next})

	require.NotNil(t, item.Data.LastRunAt)
	require.NotNil(t, item.Data.NextRunAt)
	assert.Equal(t, last, *item.Data.LastRunAt)
	assert.Equal(t, next, *item.Data.NextRunAt)
}

func TestGetObjectScheduleRejectsInvalidKind(t *testing.T) {
	ctx, rec := newRootRequestContext()
	err := (&DaemonAPI{}).GetObjectSchedule(ctx, "ns1", naming.Kind("invalid"), "app")

	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetObjectScheduleReturnsNotFoundForUnknownObject(t *testing.T) {
	ctx, rec := newRootRequestContext()
	err := (&DaemonAPI{}).GetObjectSchedule(ctx, "ns1", naming.KindSvc, "object-that-does-not-exist")

	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func newRootRequestContext() (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.Set("grants", rbac.NewGrants("root"))
	return ctx, rec
}
