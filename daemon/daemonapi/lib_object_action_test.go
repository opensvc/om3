package daemonapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/session"
	"github.com/opensvc/om3/v3/util/plog"
)

// An instance monitor not reading a request in time, as one running an
// action it reads no request until the end of, reads it later: the request
// is answered queued, with the id the monitor takes it on or refuses it
// under, which "wait" and "daemon orchestration list" answer about.
func TestJSONFromSetInstanceMonitorErrorTimeout(t *testing.T) {
	p := naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "svc1"}
	node := "node1"
	globalExpect := instance.MonitorGlobalExpectPlaced
	abort := instance.MonitorGlobalExpectAborted

	send := func(globalExpect instance.MonitorGlobalExpect) (*httptest.ResponseRecorder, uuid.UUID) {
		value := instance.MonitorUpdate{GlobalExpect: &globalExpect, CandidateOrchestrationID: uuid.New()}
		rec := httptest.NewRecorder()
		eCtx := echo.New().NewContext(httptest.NewRequest(http.MethodPost, "/", nil), rec)
		eCtx.Set("logger", plog.NewDefaultLogger())
		assert.NoError(t, JSONFromSetInstanceMonitorError(eCtx, p, node, &value, context.DeadlineExceeded))
		return rec, value.CandidateOrchestrationID
	}
	assertQueued := func(t *testing.T, rec *httptest.ResponseRecorder, id uuid.UUID) {
		assert.Equal(t, http.StatusOK, rec.Code)
		var answer api.OrchestrationQueued
		assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &answer))
		assert.Equal(t, id, answer.OrchestrationID)
		assert.Equal(t, api.OrchestrationStatusQueued, answer.Status())
		_, ok := session.GetOrchestration(id.String())
		assert.True(t, ok, "the id answered is known before the monitor reads the request")
	}
	defer instance.MonitorData.Unset(p, node)

	t.Run("an instance monitor running an action", func(t *testing.T) {
		instance.MonitorData.Set(p, node, &instance.Monitor{State: instance.MonitorStateStopProgress})
		rec, id := send(globalExpect)
		assertQueued(t, rec, id)
	})

	t.Run("an abort to an instance monitor running an action", func(t *testing.T) {
		instance.MonitorData.Set(p, node, &instance.Monitor{State: instance.MonitorStateStopProgress})
		rec, id := send(abort)
		assertQueued(t, rec, id)
	})

	t.Run("an idle instance monitor", func(t *testing.T) {
		instance.MonitorData.Set(p, node, &instance.Monitor{State: instance.MonitorStateIdle})
		rec, id := send(globalExpect)
		assertQueued(t, rec, id)
	})
}

// A node monitor not reading a request in time, as one setting the
// networks up after the daemon start, reads it later: the request is
// answered queued, with its id, where it was answered rejected while the
// node monitor applied it a moment later.
func TestJSONFromSetNodeMonitorErrorTimeout(t *testing.T) {
	globalExpect := node.MonitorGlobalExpectUnfrozen
	value := node.MonitorUpdate{GlobalExpect: &globalExpect, CandidateOrchestrationID: uuid.New()}
	rec := httptest.NewRecorder()
	eCtx := echo.New().NewContext(httptest.NewRequest(http.MethodPost, "/", nil), rec)
	eCtx.Set("logger", plog.NewDefaultLogger())
	assert.NoError(t, JSONFromSetNodeMonitorError(eCtx, "node1", &value, context.DeadlineExceeded))
	assert.Equal(t, http.StatusOK, rec.Code)
	var answer api.OrchestrationQueued
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &answer))
	assert.Equal(t, value.CandidateOrchestrationID, answer.OrchestrationID)
	assert.Equal(t, api.OrchestrationStatusQueued, answer.Status())

	rec = httptest.NewRecorder()
	eCtx = echo.New().NewContext(httptest.NewRequest(http.MethodPost, "/", nil), rec)
	assert.NoError(t, JSONFromSetNodeMonitorError(eCtx, "node1", &value, nil))
	answer = api.OrchestrationQueued{}
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &answer))
	assert.Equal(t, api.OrchestrationStatusAccepted, answer.Status(), "read in time")
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

// An action the instance monitor refuses is answered with the id its refusal
// is recorded under, in the refused state with the reason, for "daemon
// orchestration list" and "wait" to answer about it.
func TestJSONFromSetInstanceMonitorErrorHandsTheRefusalID(t *testing.T) {
	p := naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "svc1"}
	globalExpect := instance.MonitorGlobalExpectPlacedAt
	value := instance.MonitorUpdate{GlobalExpect: &globalExpect, CandidateOrchestrationID: uuid.New()}

	rec := httptest.NewRecorder()
	eCtx := echo.New().NewContext(httptest.NewRequest(http.MethodPost, "/", nil), rec)
	reason := errors.New("no destination node candidate")
	assert.NoError(t, JSONFromSetInstanceMonitorError(eCtx, p, "node1", &value, reason))
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), `"orchestration_id":"`+value.CandidateOrchestrationID.String()+`"`)
	assert.Contains(t, rec.Body.String(), "no destination node candidate")

	o, ok := session.GetOrchestration(value.CandidateOrchestrationID.String())
	assert.True(t, ok, "the id answered is known")
	assert.Equal(t, session.StateRefused, o.State)
	assert.Equal(t, "no destination node candidate", o.Error)
	assert.Equal(t, "placed@", o.Expect)
	assert.Equal(t, "test/svc/svc1", o.Path)
}
