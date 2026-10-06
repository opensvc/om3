package daemonapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/session"
)

func TestJSONFromSetInstanceMonitorErrorTimeout(t *testing.T) {
	p := naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "svc1"}
	node := "node1"
	globalExpect := instance.MonitorGlobalExpectPlaced
	value := instance.MonitorUpdate{GlobalExpect: &globalExpect, CandidateOrchestrationID: uuid.New()}

	abort := instance.MonitorGlobalExpectAborted
	abortValue := instance.MonitorUpdate{GlobalExpect: &abort, CandidateOrchestrationID: uuid.New()}

	send := func(value *instance.MonitorUpdate) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		eCtx := echo.New().NewContext(httptest.NewRequest(http.MethodPost, "/", nil), rec)
		assert.NoError(t, JSONFromSetInstanceMonitorError(eCtx, p, node, value, context.DeadlineExceeded))
		return rec
	}
	defer instance.MonitorData.Unset(p, node)

	t.Run("an instance monitor running an action", func(t *testing.T) {
		instance.MonitorData.Set(p, node, &instance.Monitor{State: instance.MonitorStateStopProgress})
		rec := send(&value)
		assert.Equal(t, http.StatusRequestTimeout, rec.Code)
		assert.Contains(t, rec.Body.String(), "test/svc/svc1 is stopping on node1: the instance monitor reads the request when the action ends")
	})

	t.Run("an abort to an instance monitor running an action is queued", func(t *testing.T) {
		instance.MonitorData.Set(p, node, &instance.Monitor{State: instance.MonitorStateStopProgress})
		rec := send(&abortValue)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), abortValue.CandidateOrchestrationID.String())
	})

	t.Run("an idle instance monitor", func(t *testing.T) {
		instance.MonitorData.Set(p, node, &instance.Monitor{State: instance.MonitorStateIdle})
		rec := send(&value)
		assert.Equal(t, http.StatusRequestTimeout, rec.Code)
		assert.Contains(t, rec.Body.String(), "timeout publishing")
	})
}

func TestJSONFromSetInstanceMonitorErrorRecordsTheOrchestration(t *testing.T) {
	p := naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "svc1"}
	globalExpect := instance.MonitorGlobalExpectPlaced
	value := instance.MonitorUpdate{GlobalExpect: &globalExpect, CandidateOrchestrationID: uuid.New()}

	rec := httptest.NewRecorder()
	eCtx := echo.New().NewContext(httptest.NewRequest(http.MethodPost, "/", nil), rec)
	assert.NoError(t, JSONFromSetInstanceMonitorError(eCtx, p, "node1", &value, nil))
	assert.Equal(t, http.StatusOK, rec.Code)

	o, ok := session.GetOrchestration(value.CandidateOrchestrationID.String())
	assert.True(t, ok, "the id answered is known before the monitor says it took it on")
	assert.Equal(t, "test/svc/svc1", o.Path)
	assert.Equal(t, "placed", o.Expect)
}
