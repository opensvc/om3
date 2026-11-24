package daemonapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/clusterdump"
	"github.com/opensvc/om3/v3/core/event"
	"github.com/opensvc/om3/v3/core/event/sseevent"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/daemondata"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// serveEvents serves an events stream of params, with the cluster data data
// replayed, and returns its header and its events.
func serveEvents(t *testing.T, data *clusterdump.Data, params api.GetDaemonEventsParams) (http.Header, []event.Event) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bus := pubsub.NewBus(t.Name())
	bus.Start(ctx)

	// The cluster data manager answering the snapshot requests.
	sub := bus.Sub(t.Name() + " daemondata")
	sub.AddFilter(&msgbus.ClusterDataSnapshotRequest{})
	sub.Start()
	defer func() { _ = sub.Stop() }()
	go func() {
		for i := range sub.C {
			if r, ok := i.(*msgbus.ClusterDataSnapshotRequest); ok {
				r.ReplyC <- data.DeepCopy()
			}
		}
	}()

	a := &DaemonAPI{localhost: "node1", Bus: bus, Daemondata: &daemondata.T{}, SubQS: pubsub.WithQueueSize(100)}
	reqCtx, reqCancel := context.WithTimeout(ctx, 5*time.Second)
	defer reqCancel()
	rec := httptest.NewRecorder()
	ectx := echo.New().NewContext(httptest.NewRequest(http.MethodGet, "/", nil).WithContext(reqCtx), rec)
	ectx.Set("grants", rbac.Grants{rbac.GrantRoot})
	ectx.Set("logger", plog.NewDefaultLogger())
	require.NoError(t, a.GetDaemonEvents(ectx, "node1", params))

	r := sseevent.NewReadCloser(io.NopCloser(rec.Body))
	var l []event.Event
	for {
		ev, err := r.Read()
		if err != nil || ev == nil {
			break
		}
		l = append(l, *ev)
	}
	return rec.Header(), l
}

func newReplayData(paths ...string) *clusterdump.Data {
	data := clusterdump.NewData("node1")
	for _, p := range paths {
		data.Cluster.Object[p] = object.Status{}
	}
	return data
}

func pointer[T any](v T) *T {
	return &v
}

// The replay of a stream asked with replay_done ends with a ReplayDone event,
// in the id sequence, carrying the count of the events replayed, and the
// header tells it.
func TestReplayDone(t *testing.T) {
	header, events := serveEvents(t, newReplayData("ns1/svc/a", "ns1/svc/b"), api.GetDaemonEventsParams{
		Filter:     &[]string{"ObjectStatusUpdated"},
		Replay:     pointer(true),
		ReplayDone: pointer(true),
		Limit:      pointer(int64(3)),
	})
	assert.Equal(t, "true", header.Get(api.HeaderReplayDone))
	require.Len(t, events, 3)
	assert.Equal(t, "ObjectStatusUpdated", events[0].Kind)
	assert.Equal(t, "ObjectStatusUpdated", events[1].Kind)
	assert.Equal(t, event.KindReplayDone, events[2].Kind)
	for i, ev := range events {
		assert.Equal(t, uint64(i), ev.ID, "the ReplayDone event is in the id sequence")
	}
	var done event.ReplayDone
	require.NoError(t, json.Unmarshal(events[2].Data, &done))
	assert.Equal(t, uint64(2), done.Count)
}

// An empty replay ends with a ReplayDone event too.
func TestReplayDoneOfAnEmptyReplay(t *testing.T) {
	header, events := serveEvents(t, newReplayData(), api.GetDaemonEventsParams{
		Filter:     &[]string{"ObjectStatusUpdated"},
		Replay:     pointer(true),
		ReplayDone: pointer(true),
		Limit:      pointer(int64(1)),
	})
	assert.Equal(t, "true", header.Get(api.HeaderReplayDone))
	require.Len(t, events, 1)
	assert.Equal(t, event.KindReplayDone, events[0].Kind)
	var done event.ReplayDone
	require.NoError(t, json.Unmarshal(events[0].Data, &done))
	assert.Equal(t, uint64(0), done.Count)
}

// Without replay_done, or without replay, no ReplayDone event is sent, and the
// header does not tell one.
func TestNoReplayDone(t *testing.T) {
	header, events := serveEvents(t, newReplayData("ns1/svc/a"), api.GetDaemonEventsParams{
		Filter:   &[]string{"ObjectStatusUpdated"},
		Replay:   pointer(true),
		Duration: pointer("300ms"),
	})
	assert.Empty(t, header.Get(api.HeaderReplayDone))
	require.Len(t, events, 1)
	assert.Equal(t, "ObjectStatusUpdated", events[0].Kind)

	header, events = serveEvents(t, newReplayData("ns1/svc/a"), api.GetDaemonEventsParams{
		Filter:     &[]string{"ObjectStatusUpdated"},
		ReplayDone: pointer(true),
		Duration:   pointer("300ms"),
	})
	assert.Empty(t, header.Get(api.HeaderReplayDone))
	assert.Empty(t, events)
}
