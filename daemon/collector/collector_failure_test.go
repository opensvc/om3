package collector

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/plog"
)

func TestCollectorFailureUpdate(t *testing.T) {
	log := plog.NewDefaultLogger()
	f := newCollectorFailure("resource info")
	now := time.Now()
	down := errors.New("POST /api/instance/resource_info unexpected status code: wanted 202 got 502")

	f.update(log, now, down, 3)
	assert.True(t, f.isFailing(), "warned at once")
	assert.Equal(t, warnBackoffMin, f.backoff.interval)

	f.update(log, now.Add(time.Second), down, 3)
	assert.Equal(t, warnBackoffMin, f.backoff.interval, "not warned again before the interval")

	f.update(log, now.Add(warnBackoffMin), down, 3)
	assert.Equal(t, 2*warnBackoffMin, f.backoff.interval, "warned again, the interval doubled")

	f.update(log, now.Add(time.Minute), nil, 0)
	assert.False(t, f.isFailing(), "a success starts the pacing over")

	f.update(log, now.Add(time.Minute), down, 0)
	assert.Equal(t, warnBackoffMin, f.backoff.interval)
	f.reset()
	assert.False(t, f.isFailing())
}

func TestIsRefusedStatus(t *testing.T) {
	for code, want := range map[int]bool{
		http.StatusBadRequest:          true,
		http.StatusForbidden:           true,
		http.StatusNotFound:            true,
		http.StatusAccepted:            false,
		http.StatusInternalServerError: false,
		http.StatusBadGateway:          false,
	} {
		assert.Equal(t, want, isRefusedStatus(code), "%d", code)
	}
}

// TestSendResInfoChangeStopsAtTheFirstFailure pins that a collector down
// costs one fetch of resource info, and one send, per tick, and that the
// queue is kept for the next tick.
func TestSendResInfoChangeStopsAtTheFirstFailure(t *testing.T) {
	testhelper.Setup(t)
	t.Cleanup(func() { rawconfig.Load(map[string]string{}) })

	var fetches int
	down := true
	f := &fakeRequester{respond: func(*http.Request) *http.Response {
		if down {
			return response(http.StatusBadGateway, ``)
		}
		return response(http.StatusAccepted, `null`)
	}}
	tr := &T{
		ctx:            context.Background(),
		log:            plog.NewDefaultLogger(),
		localhost:      "node1",
		client:         f,
		resInfoSent:    make(map[string]resInfoSent),
		resInfoToSend:  make(map[string]*msgbus.InstanceResourceInfoUpdated),
		resInfoFailure: newCollectorFailure("resource info"),
		resInfoGet: func(p naming.Path, _ string) (resource.Infos, error) {
			fetches++
			return resource.NewInfos(p), nil
		},
	}
	for _, name := range []string{"s1", "s2", "s3"} {
		p := naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: name}
		tr.resInfoToSend[p.String()+"@node2"] = &msgbus.InstanceResourceInfoUpdated{Path: p, Node: "node2"}
	}

	tr.sendResInfoChange()
	assert.Equal(t, 1, fetches, "one fetch")
	assert.Len(t, f.requests, 1, "one send")
	assert.Len(t, tr.resInfoToSend, 3, "all stay queued")
	assert.True(t, tr.resInfoFailure.isFailing())

	down = false
	tr.sendResInfoChange()
	assert.Len(t, f.requests, 4)
	assert.Empty(t, tr.resInfoToSend)
	assert.False(t, tr.resInfoFailure.isFailing(), "the collector accepts again")
}

// TestPostPingNotSent pins that a ping not sent says nothing of the
// collector health, unlike a ping failing.
func TestPostPingNotSent(t *testing.T) {
	tr := &T{
		ctx:          context.Background(),
		log:          plog.NewDefaultLogger(),
		pingInterval: time.Hour,
	}
	tr.initChangesForTest()
	assert.ErrorIs(t, tr.postPing(), errCollectorDataNotSent, "no client")

	tr.client = &fakeRequester{respond: func(*http.Request) *http.Response {
		return response(http.StatusBadGateway, ``)
	}}
	tr.feedPingOrStatusAt = time.Now()
	assert.ErrorIs(t, tr.postPing(), errCollectorDataNotSent, "throttled")

	tr.feedPingOrStatusAt = time.Time{}
	err := tr.postPing()
	require.Error(t, err)
	assert.NotErrorIs(t, err, errCollectorDataNotSent, "a ping failing is a failure")
}

func (t *T) initChangesForTest() {
	t.instances = make(map[string]struct{})
	t.changes = changesData{
		instanceStatusUpdates: make(map[string]*msgbus.InstanceStatusUpdated),
		instanceStatusDeletes: make(map[string]*msgbus.InstanceStatusDeleted),
	}
	t.daemonStatusChange = make(map[string]struct{})
}
