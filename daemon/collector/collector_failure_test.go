package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
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
		http.StatusUnauthorized:        false,
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
		sendResultC:    make(chan sendResult, sendResultChanSize),
		postTimeout:    time.Second,
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
	applyNextResult(t, tr)
	assert.Equal(t, 1, fetches, "one fetch")
	assert.Len(t, f.requests, 1, "one send")
	assert.Len(t, tr.resInfoToSend, 3, "all stay queued")
	assert.True(t, tr.resInfoFailure.isFailing())

	down = false
	tr.sendResInfoChange()
	applyNextResult(t, tr)
	assert.Len(t, f.requests, 4)
	assert.Empty(t, tr.resInfoToSend)
	assert.False(t, tr.resInfoFailure.isFailing(), "the collector accepts again")
}

func (t *T) initChangesForTest() {
	t.daemonStatusChange = make(map[string]struct{})
}

// A 401 refuses the credentials of the speaker, not the resource info: it is
// kept queued, for the next send, by the next speaker if need be.
func TestAResourceInfoAnswered401IsRetried(t *testing.T) {
	f := &fakeRequester{respond: func(*http.Request) *http.Response {
		return response(http.StatusUnauthorized, "")
	}}
	v := &msgbus.InstanceResourceInfoUpdated{Path: naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "s1"}, Node: "node1"}
	outcome, err := doPostResInfo(context.Background(), f, time.Second, v, resource.Infos{})
	assert.Equal(t, sendFailed, outcome)
	assert.NotErrorIs(t, err, errCollectorRefused)

	f.respond = func(*http.Request) *http.Response { return response(http.StatusForbidden, "") }
	outcome, err = doPostResInfo(context.Background(), f, time.Second, v, resource.Infos{})
	assert.Equal(t, sendDropped, outcome, "a 403 refuses the resource info for good")
	assert.ErrorIs(t, err, errCollectorRefused)
}

// A series of like warnings logs the first one at the warn level, the next
// ones at the debug level, and their count at the end.
func TestWarnSeries(t *testing.T) {
	var buf bytes.Buffer
	w := warnSeries{log: plog.NewLogger(zerolog.New(&buf).Level(zerolog.TraceLevel))}
	for _, i := range []string{"a", "b", "c"} {
		w.add("refused %s", i)
	}
	w.done("refused items")

	type entry struct {
		Level   string `json:"level"`
		Message string `json:"message"`
	}
	var got []entry
	d := json.NewDecoder(&buf)
	for d.More() {
		var e entry
		require.NoError(t, d.Decode(&e))
		got = append(got, e)
	}
	assert.Equal(t, []entry{
		{"warn", "refused a"},
		{"debug", "refused b"},
		{"debug", "refused c"},
		{"warn", "refused items: 3 times, the first one logged, the others at the debug level"},
	}, got)

	buf.Reset()
	one := warnSeries{log: plog.NewLogger(zerolog.New(&buf).Level(zerolog.TraceLevel))}
	one.add("refused a")
	one.done("refused items")
	assert.Equal(t, 1, strings.Count(buf.String(), "\n"), "no count for a single warning")
}
