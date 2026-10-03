package collector

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/daemon/msgbus"
)

func TestFeedOf(t *testing.T) {
	for path, want := range map[string]string{
		"/feeder/api/daemon/ping":            feedDaemonPing,
		"/api/daemon/status":                 feedDaemonStatus,
		"/api/daemon/change":                 feedDaemonChange,
		"/feeder/api/instance/resource_info": feedResourceInfo,
		"/feeder/api/object/config":          feedObjectConfig,
		"/feeder/api/instance/action":        feedInstanceAction,
		"/feeder/api/node/system":            feedOther,
	} {
		assert.Equal(t, want, feedOf(path), path)
	}
}

// failingRequester answers every call with a transport error.
type failingRequester struct {
	fakeRequester
}

func (f *failingRequester) Do(*http.Request) (*http.Response, error) {
	return nil, errors.New("connection refused")
}

func TestInstrumentedRequesterCounts(t *testing.T) {
	count := func(feed, code string) float64 {
		return value(collectorRequestsTotal.WithLabelValues(feed, code))
	}
	accepted0, failed0 := count(feedDaemonPing, "202"), count(feedDaemonPing, "error")

	r := newInstrumentedRequester(&fakeRequester{respond: func(*http.Request) *http.Response {
		return response(http.StatusAccepted, `null`)
	}})
	req, err := r.NewRequestWithContext(context.Background(), http.MethodPost, "/api/daemon/ping", strings.NewReader("{}"))
	require.NoError(t, err)
	_, err = r.Do(req)
	require.NoError(t, err)
	assert.Equal(t, accepted0+1, count(feedDaemonPing, "202"))

	r = newInstrumentedRequester(&failingRequester{})
	req, err = r.NewRequestWithContext(context.Background(), http.MethodPost, "/api/daemon/ping", strings.NewReader("{}"))
	require.NoError(t, err)
	_, err = r.Do(req)
	require.Error(t, err)
	assert.Equal(t, failed0+1, count(feedDaemonPing, "error"), "a call without response counts as error")
}

func TestSetPendingMetrics(t *testing.T) {
	pending := func(feed string) float64 {
		return value(collectorPending.WithLabelValues(feed))
	}
	tr, _ := newTestT(t)
	tr.initChangesForTest()
	tr.resInfoToSend = make(map[string]*msgbus.InstanceResourceInfoUpdated)
	tr.daemonStatusChange["@node1"] = struct{}{}
	tr.daemonStatusChange["test/svc/s1"] = struct{}{}
	tr.resInfoToSend["test/svc/s1@node1"] = &msgbus.InstanceResourceInfoUpdated{}
	tr.isSpeaker = true
	tr.onInstanceActionPending(pendingMsg(newTestCollectorAction(), "node2"))
	tr.onInstanceActionPending(pendingMsg(endedAction(newTestCollectorAction(), "ok"), "node2"))

	tr.setPendingMetrics()
	assert.Equal(t, 2.0, pending(feedDaemonStatus))
	assert.Equal(t, 0.0, pending(feedDaemonChange))
	assert.Equal(t, 1.0, pending(feedResourceInfo))
	assert.Equal(t, 0.0, pending(feedObjectConfig))
	assert.Equal(t, 2.0, pending(feedInstanceAction))

	tr.isSpeaker = false
	tr.setPendingMetrics()
	assert.Equal(t, 0.0, pending(feedDaemonStatus), "a node not speaker holds nothing")
	assert.Equal(t, 0.0, pending(feedInstanceAction))
}

func TestActionPendingFilesMetric(t *testing.T) {
	tr, _ := newTestT(t)
	dir := tr.actionPendingDir
	running := newTestCollectorAction()
	require.NoError(t, dir.Write(running))
	ended := newTestCollectorAction()
	require.NoError(t, dir.Write(endedAction(ended, "ok")))
	acknowledged := newTestCollectorAction()
	require.NoError(t, dir.WriteUUID(acknowledged.Key(), "oc3-uuid"))

	tr.announceActionPending(true)
	assert.Equal(t, 2.0, value(collectorActionPendingFiles), "a running action acknowledged is not pending")

	require.NoError(t, dir.RemoveAll(running.Key()))
	require.NoError(t, dir.RemoveAll(ended.Key()))
	tr.announceActionPending(false)
	assert.Equal(t, 0.0, value(collectorActionPendingFiles))
}

// value returns the value of a counter or gauge.
func value(m prometheus.Metric) float64 {
	var d dto.Metric
	if err := m.Write(&d); err != nil {
		panic(err)
	}
	if d.Counter != nil {
		return d.Counter.GetValue()
	}
	return d.Gauge.GetValue()
}
