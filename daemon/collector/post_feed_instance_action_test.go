package collector

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/oc3path"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type (
	// fakeRequester answers the collector requests with respond, and keeps
	// the requests and their bodies.
	fakeRequester struct {
		respond  func(req *http.Request) *http.Response
		requests []*http.Request
		bodies   [][]byte
	}

	fakePublisher struct {
		msgs []pubsub.Messager
	}
)

func (f *fakeRequester) URL() string { return "https://collector" }

func (f *fakeRequester) NewRequestWithContext(ctx context.Context, method string, relPath string, body io.Reader) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, method, "https://collector"+relPath, body)
}

func (f *fakeRequester) Do(req *http.Request) (*http.Response, error) {
	b, _ := io.ReadAll(req.Body)
	f.requests = append(f.requests, req)
	f.bodies = append(f.bodies, b)
	return f.respond(req), nil
}

func (f *fakePublisher) Pub(m pubsub.Messager, _ ...pubsub.Label) {
	f.msgs = append(f.msgs, m)
}

func response(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body))}
}

func newTestCollectorAction() collector.Action {
	begin := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	return collector.Action{
		Path:      naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "s1"},
		Action:    "start",
		Argv:      []string{"s1", "instance", "start"},
		RIDs:      "fs#1",
		Origin:    "user",
		SessionID: uuid.New(),
		ExecID:    uuid.New(),
		PID:       4321,
		Begin:     begin,
	}
}

func endedAction(a collector.Action, status string) collector.Action {
	a.End = a.Begin.Add(3 * time.Second)
	a.Status = status
	return a
}

func pendingMsg(a collector.Action, nodename string) *msgbus.InstanceActionPending {
	return &msgbus.InstanceActionPending{Path: a.Path, Node: nodename, Phase: a.Phase(), Action: a}
}

// journalRecord returns a journal record as journalctl -o json renders a
// zerolog record of the action process.
func journalRecord(t *testing.T, at time.Time, level, rid, message string) map[string]any {
	m := map[string]any{"time": at.Format(time.RFC3339Nano), "level": level, "message": message}
	if rid != "" {
		m["rid"] = rid
	}
	b, err := json.Marshal(m)
	require.NoError(t, err)
	return map[string]any{"MESSAGE": message, "JSON": string(b)}
}

func newTestT(t *testing.T) (*T, *fakePublisher) {
	pub := &fakePublisher{}
	tr := &T{
		log:               plog.NewDefaultLogger(),
		localhost:         "node1",
		publisher:         pub,
		version:           "3.0.0",
		actionPendingDir:  collector.ActionPendingDir(t.TempDir()),
		actionAnnouncedAt: make(map[string]time.Time),
		actionSendResultC: make(chan []actionSendResult, 1),
		actionFailure:     newCollectorFailure("action logs"),
	}
	tr.dropActionToSend()
	return tr, pub
}

func TestActionLinesFromJournal(t *testing.T) {
	at := time.Date(2026, 10, 2, 10, 0, 1, 500, time.UTC)
	records := []map[string]any{
		journalRecord(t, at, "info", "", ">>> do start"),
		journalRecord(t, at, "info", "fs#1", "mount /dev/sdb"),
		journalRecord(t, at, "info", "fs#1", "mounted"),
		journalRecord(t, at, "debug", "fs#1", "a debug line is dropped"),
		journalRecord(t, at, "warn", "fs#1", "slow mount"),
		journalRecord(t, at, "error", "ip#1", "address in use"),
		journalRecord(t, at, "info", "", "<<< done start"),
		// a record without zerolog JSON falls back to the journal fields
		{"MESSAGE": "raw", "RID": "app#1", "PRIORITY": "4"},
		{"PRIORITY": "6"},
	}
	lines := actionLinesFromJournal(records, 4321)

	want := []struct{ rid, status, log string }{
		{"", "ok", ">>> do start"},
		{"fs#1", "ok", "mount /dev/sdb\nmounted"},
		{"fs#1", "warn", "slow mount"},
		{"ip#1", "err", "address in use"},
		{"", "ok", "<<< done start"},
		{"app#1", "warn", "raw"},
	}
	require.Len(t, lines, len(want))
	for i, w := range want {
		assert.Equal(t, w.rid, lines[i].RID, "line %d rid", i)
		assert.Equal(t, w.status, lines[i].Status, "line %d status", i)
		assert.Equal(t, w.log, lines[i].StatusLog, "line %d status_log", i)
		assert.Equal(t, "4321", lines[i].PID, "line %d pid", i)
		assert.Equal(t, "", lines[i].Subset, "line %d subset", i)
	}
	assert.Equal(t, at.Local().Format("2006-01-02 15:04:05"), lines[0].Begin, "the om2 time format, local")
}

func TestTrimActionLineMessage(t *testing.T) {
	short := strings.Repeat("a", actionLineMaxLen)
	assert.Equal(t, short, trimActionLineMessage(short))

	long := strings.Repeat("h", actionLineMaxLen) + strings.Repeat("t", actionLineMaxLen)
	trimmed := trimActionLineMessage(long)
	assert.Len(t, trimmed, actionLineMaxLen)
	assert.True(t, strings.HasPrefix(trimmed, strings.Repeat("h", actionLineMaxLen/2)+actionLineTrimTag))
	assert.True(t, strings.HasSuffix(trimmed, "t"))
}

func TestNewActionPost(t *testing.T) {
	a := newTestCollectorAction()

	begin := newActionPost("node2", a, collector.ActionPhaseBegin, "", "3.0.0", nil)
	assert.Equal(t, "test/svc/s1", begin.Path)
	assert.Equal(t, "node2", begin.Nodename, "the node the action ran on, not the speaker")
	assert.Equal(t, "4321", begin.PID)
	assert.Equal(t, a.SessionID.String(), begin.SessionUUID)
	assert.Equal(t, "2026-10-02T10:00:00Z", begin.Begin)
	assert.Empty(t, begin.End, "a begin has no end")
	assert.Empty(t, begin.Status)
	assert.Empty(t, begin.UUID, "the collector assigns the uuid of a begin")
	assert.NotNil(t, begin.Lines)
	assert.False(t, begin.Cron)

	ended := endedAction(a, "err")
	end := newActionPost("node2", ended, collector.ActionPhaseEnd, "oc3-uuid", "3.0.0", nil)
	assert.Equal(t, "2026-10-02T10:00:03Z", end.End)
	assert.Equal(t, "err", end.Status)
	assert.Equal(t, "oc3-uuid", end.UUID, "the end sends back the begin uuid")

	end = newActionPost("node2", ended, collector.ActionPhaseEnd, "", "3.0.0", nil)
	assert.Equal(t, a.ExecID.String(), end.UUID, "the exec id stands for an unknown begin uuid")
}

func TestSendAction(t *testing.T) {
	a := newTestCollectorAction()
	at := time.Date(2026, 10, 2, 10, 0, 1, 0, time.UTC)
	readLog := func(_ context.Context, nodename string, got collector.Action) ([]map[string]any, error) {
		assert.Equal(t, "node2", nodename, "the log is read on the node the action ran on")
		assert.Equal(t, a.ExecID, got.ExecID)
		return []map[string]any{journalRecord(t, at, "info", "fs#1", "mounted")}, nil
	}

	t.Run("begin accepted returns the collector uuid", func(t *testing.T) {
		f := &fakeRequester{respond: func(*http.Request) *http.Response {
			return response(http.StatusAccepted, `{"uuid":"oc3-uuid"}`)
		}}
		job := actionSendJob{key: a.Key(), msg: pendingMsg(a, "node2")}
		r := sendAction(context.Background(), f, readLog, "3.0.0", job)
		require.NoError(t, r.err)
		assert.True(t, r.done)
		assert.Equal(t, "oc3-uuid", r.uuid)
		require.Len(t, f.requests, 1)
		assert.Equal(t, http.MethodPost, f.requests[0].Method)
		assert.Equal(t, oc3path.FeedInstanceAction, f.requests[0].URL.Path)
		var body map[string]any
		require.NoError(t, json.Unmarshal(f.bodies[0], &body))
		assert.Equal(t, "", body["end"])
		assert.Equal(t, "3.0.0", body["version"])
		assert.Equal(t, "node2", body["nodename"])
	})

	t.Run("end carries the begin uuid and the log lines", func(t *testing.T) {
		f := &fakeRequester{respond: func(*http.Request) *http.Response {
			return response(http.StatusAccepted, `null`)
		}}
		ended := endedAction(a, "ok")
		job := actionSendJob{key: a.Key(), msg: pendingMsg(ended, "node2"), uuid: "oc3-uuid"}
		r := sendAction(context.Background(), f, readLog, "3.0.0", job)
		require.NoError(t, r.err)
		assert.True(t, r.done)
		require.Len(t, f.requests, 1)
		assert.Equal(t, http.MethodPut, f.requests[0].Method)
		var body actionPost
		require.NoError(t, json.Unmarshal(f.bodies[0], &body))
		assert.Equal(t, "oc3-uuid", body.UUID)
		assert.Equal(t, "ok", body.Status)
		require.Len(t, body.Lines, 1)
		assert.Equal(t, "mounted", body.Lines[0].StatusLog)
	})

	t.Run("end sent without lines when the log is unreadable", func(t *testing.T) {
		f := &fakeRequester{respond: func(*http.Request) *http.Response {
			return response(http.StatusAccepted, `null`)
		}}
		failingLog := func(context.Context, string, collector.Action) ([]map[string]any, error) {
			return nil, errors.New("no journal")
		}
		job := actionSendJob{key: a.Key(), msg: pendingMsg(endedAction(a, "ok"), "node2")}
		r := sendAction(context.Background(), f, failingLog, "3.0.0", job)
		assert.ErrorContains(t, r.err, "no journal", "the log error is reported")
		assert.True(t, r.done, "but does not hold the end")
		var body actionPost
		require.NoError(t, json.Unmarshal(f.bodies[0], &body))
		assert.NotNil(t, body.Lines)
		assert.Empty(t, body.Lines)
	})

	t.Run("refused is done", func(t *testing.T) {
		f := &fakeRequester{respond: func(*http.Request) *http.Response {
			return response(http.StatusBadRequest, `{"title":"bad"}`)
		}}
		r := sendAction(context.Background(), f, readLog, "3.0.0", actionSendJob{key: a.Key(), msg: pendingMsg(a, "node2")})
		assert.True(t, r.done, "sending it again would not do better")
		assert.Error(t, r.err)
	})

	t.Run("node unknown in the cluster is done", func(t *testing.T) {
		f := &fakeRequester{respond: func(*http.Request) *http.Response {
			return response(http.StatusForbidden, `{"detail":"node2: not a node of the cluster"}`)
		}}
		r := sendAction(context.Background(), f, readLog, "3.0.0", actionSendJob{key: a.Key(), msg: pendingMsg(a, "node2")})
		assert.True(t, r.done, "sending it again would not do better")
		assert.Error(t, r.err)
	})

	t.Run("server error is retried", func(t *testing.T) {
		f := &fakeRequester{respond: func(*http.Request) *http.Response {
			return response(http.StatusInternalServerError, ``)
		}}
		r := sendAction(context.Background(), f, readLog, "3.0.0", actionSendJob{key: a.Key(), msg: pendingMsg(a, "node2")})
		assert.False(t, r.done)
		assert.Error(t, r.err)
	})
}

func TestOnInstanceActionPendingQueue(t *testing.T) {
	tr, pub := newTestT(t)
	a := newTestCollectorAction()
	key := a.Key()

	tr.onInstanceActionPending(pendingMsg(a, "node2"))
	assert.Empty(t, tr.actionToSend, "only the speaker queues")

	tr.isSpeaker = true
	tr.onInstanceActionPending(pendingMsg(a, "node2"))
	require.Contains(t, tr.actionToSend, key)
	assert.NotNil(t, tr.actionToSend[key].begin)

	ended := endedAction(a, "ok")
	tr.onInstanceActionPending(pendingMsg(ended, "node2"))
	assert.Nil(t, tr.actionToSend[key].begin, "the end replaces its begin")
	assert.NotNil(t, tr.actionToSend[key].end)

	tr.onInstanceActionPending(pendingMsg(a, "node2"))
	assert.Nil(t, tr.actionToSend[key].begin, "a begin announced after its end is ignored")

	// The collector has the end: an announce whose acknowledgement was
	// lost is acknowledged again, not sent again.
	delete(tr.actionToSend, key)
	tr.actionSent[key] = actionSentTrace{phase: collector.ActionPhaseEnd, at: time.Now()}
	tr.onInstanceActionPending(pendingMsg(ended, "node2"))
	assert.NotContains(t, tr.actionToSend, key)
	require.Len(t, pub.msgs, 1)
	sent := pub.msgs[0].(*msgbus.InstanceActionSent)
	assert.Equal(t, collector.ActionPhaseEnd, sent.Phase)
	assert.Equal(t, "node2", sent.Node)
	assert.Equal(t, a.ExecID, sent.ExecID)

	// A begin sent, then its end announced: the end is queued.
	other := newTestCollectorAction()
	tr.actionSent[other.Key()] = actionSentTrace{phase: collector.ActionPhaseBegin, uuid: "u", at: time.Now()}
	tr.onInstanceActionPending(pendingMsg(endedAction(other, "ok"), "node2"))
	require.Contains(t, tr.actionToSend, other.Key())
	assert.NotNil(t, tr.actionToSend[other.Key()].end)

	assert.NotContains(t, tr.actionAnnouncedAt, key, "a peer action is not announced here")
	tr.onInstanceActionPending(pendingMsg(newTestCollectorAction(), "node1"))
	assert.Len(t, tr.actionAnnouncedAt, 1, "a local action announce is recorded")
}

func TestOnActionSendResults(t *testing.T) {
	tr, pub := newTestT(t)
	tr.isSpeaker = true
	a := newTestCollectorAction()
	key := a.Key()
	begin := pendingMsg(a, "node2")
	tr.actionToSend[key] = &actionToSend{begin: begin}
	tr.actionSending = true

	tr.onActionSendResults([]actionSendResult{{
		actionSendJob: actionSendJob{key: key, msg: begin, uuid: "oc3-uuid"},
		done:          true,
	}})
	assert.False(t, tr.actionSending)
	assert.NotContains(t, tr.actionToSend, key)
	assert.Equal(t, "oc3-uuid", tr.actionBeginUUID[key])
	require.Len(t, pub.msgs, 1)
	sent := pub.msgs[0].(*msgbus.InstanceActionSent)
	assert.Equal(t, collector.ActionPhaseBegin, sent.Phase)
	assert.Equal(t, "oc3-uuid", sent.UUID)

	// the end then goes with the begin uuid
	end := pendingMsg(endedAction(a, "ok"), "node2")
	tr.onInstanceActionPending(end)
	jobs := tr.nextActionJobs()
	require.Len(t, jobs, 1)
	assert.Equal(t, collector.ActionPhaseEnd, jobs[0].msg.Phase)
	assert.Equal(t, "oc3-uuid", jobs[0].uuid)

	// a failed send stays queued
	tr.onActionSendResults([]actionSendResult{{
		actionSendJob: actionSendJob{key: key, msg: end},
		err:           errors.New("timeout"),
	}})
	assert.Contains(t, tr.actionToSend, key)
	assert.Len(t, pub.msgs, 1, "nothing acknowledged")

	tr.onActionSendResults([]actionSendResult{{
		actionSendJob: actionSendJob{key: key, msg: end, uuid: "oc3-uuid"},
		done:          true,
	}})
	assert.NotContains(t, tr.actionToSend, key)
	assert.NotContains(t, tr.actionBeginUUID, key)
	require.Len(t, pub.msgs, 2)
	assert.Equal(t, collector.ActionPhaseEnd, pub.msgs[1].(*msgbus.InstanceActionSent).Phase)
}

func TestSendActionsBatch(t *testing.T) {
	tr, pub := newTestT(t)
	tr.isSpeaker = true
	tr.ctx = context.Background()
	f := &fakeRequester{respond: func(req *http.Request) *http.Response {
		if req.Method == http.MethodPost {
			return response(http.StatusAccepted, `{"uuid":"oc3-uuid"}`)
		}
		return response(http.StatusAccepted, `null`)
	}}
	tr.client = f
	a := newTestCollectorAction()
	tr.onInstanceActionPending(pendingMsg(a, "node1"))

	tr.sendActions()
	assert.True(t, tr.actionSending)
	tr.sendActions()
	select {
	case results := <-tr.actionSendResultC:
		tr.onActionSendResults(results)
	case <-time.After(5 * time.Second):
		t.Fatal("no send result")
	}
	assert.Len(t, f.requests, 1, "one batch at a time")
	assert.Equal(t, "oc3-uuid", tr.actionBeginUUID[a.Key()])
	require.Len(t, pub.msgs, 1)
}

func TestAnnounceActionPending(t *testing.T) {
	tr, pub := newTestT(t)
	dir := tr.actionPendingDir
	old := time.Now().Add(-2 * actionAnnounceInterval)
	setModTime := func(key string, suffixes ...string) {
		for _, suffix := range suffixes {
			require.NoError(t, os.Chtimes(string(dir)+"/"+key+suffix, old, old))
		}
	}

	running := newTestCollectorAction()
	require.NoError(t, dir.Write(running))
	setModTime(running.Key(), ".begin.json")

	ended := newTestCollectorAction()
	require.NoError(t, dir.Write(ended))
	require.NoError(t, dir.Write(endedAction(ended, "ok")))
	require.NoError(t, dir.WriteUUID(ended.Key(), "oc3-uuid"))
	setModTime(ended.Key(), ".begin.json", ".end.json", ".uuid")

	acknowledged := newTestCollectorAction()
	require.NoError(t, dir.WriteUUID(acknowledged.Key(), "oc3-uuid"))
	setModTime(acknowledged.Key(), ".uuid")

	fresh := newTestCollectorAction()
	require.NoError(t, dir.Write(fresh))

	expired := newTestCollectorAction()
	require.NoError(t, dir.Write(expired))
	tooOld := time.Now().Add(-actionPendingExpire - time.Minute)
	require.NoError(t, os.Chtimes(string(dir)+"/"+expired.Key()+".begin.json", tooOld, tooOld))

	tr.announceActionPending(false)
	announced := make(map[string]*msgbus.InstanceActionPending)
	for _, m := range pub.msgs {
		msg := m.(*msgbus.InstanceActionPending)
		assert.Equal(t, "node1", msg.Node)
		announced[msg.Action.Key()] = msg
	}
	require.Len(t, announced, 2, "%v", announced)
	assert.Equal(t, collector.ActionPhaseBegin, announced[running.Key()].Phase)
	assert.Equal(t, collector.ActionPhaseEnd, announced[ended.Key()].Phase, "the end is announced in place of its begin")
	assert.Equal(t, "oc3-uuid", announced[ended.Key()].UUID, "the end carries the begin uuid")
	assert.NotContains(t, announced, acknowledged.Key(), "a running action acknowledged has nothing to announce")
	assert.NotContains(t, announced, fresh.Key(), "a fresh action waits for its own signal")

	l, err := dir.List()
	require.NoError(t, err)
	for _, k := range l {
		assert.NotEqual(t, expired.Key(), k.Key, "the expired action is removed")
	}
	assert.Len(t, l, 4, "the acknowledged uuid is kept for the end")

	// announced keys are not announced again before the interval
	pub.msgs = nil
	for key := range announced {
		tr.actionAnnouncedAt[key] = time.Now()
	}
	tr.announceActionPending(false)
	assert.Empty(t, pub.msgs)

	// unless forced, as on a speaker change
	tr.announceActionPending(true)
	assert.Len(t, pub.msgs, 3, "the fresh action is announced too")

	// a node without collector only expires
	pub.msgs = nil
	tr.disable = true
	tr.announceActionPending(true)
	assert.Empty(t, pub.msgs)
}

func TestOnLocalInstanceActionSent(t *testing.T) {
	tr, _ := newTestT(t)
	dir := tr.actionPendingDir
	a := newTestCollectorAction()
	key := a.Key()
	require.NoError(t, dir.Write(a))

	tr.onInstanceActionSent(&msgbus.InstanceActionSent{Path: a.Path, Node: "node2", ExecID: a.ExecID, Phase: collector.ActionPhaseEnd})
	_, err := dir.Read(key, collector.ActionPhaseBegin)
	require.NoError(t, err, "an acknowledgement for another node leaves the local files")

	tr.onInstanceActionSent(&msgbus.InstanceActionSent{Path: a.Path, Node: "node1", ExecID: a.ExecID, Phase: collector.ActionPhaseBegin, UUID: "oc3-uuid"})
	_, err = dir.Read(key, collector.ActionPhaseBegin)
	assert.ErrorIs(t, err, os.ErrNotExist, "the begin acknowledged is dropped")
	s, err := dir.ReadUUID(key)
	require.NoError(t, err)
	assert.Equal(t, "oc3-uuid", s, "its collector uuid is kept for the end")

	require.NoError(t, dir.Write(endedAction(a, "ok")))
	msg, err := NewInstanceActionPending(dir, key, collector.ActionPhaseEnd, "node1")
	require.NoError(t, err)
	assert.Equal(t, "oc3-uuid", msg.UUID)

	tr.actionAnnouncedAt[key] = time.Now()
	tr.onInstanceActionSent(&msgbus.InstanceActionSent{Path: a.Path, Node: "node1", ExecID: a.ExecID, Phase: collector.ActionPhaseEnd})
	l, err := dir.List()
	require.NoError(t, err)
	assert.Empty(t, l, "the end acknowledged drops all the files")
	assert.NotContains(t, tr.actionAnnouncedAt, key)
}

func TestNewInstanceActionPendingPhaseMismatch(t *testing.T) {
	dir := collector.ActionPendingDir(t.TempDir())
	a := newTestCollectorAction()
	b, err := json.Marshal(endedAction(a, "ok"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(string(dir)+"/"+a.Key()+".begin.json", b, 0600))
	_, err = NewInstanceActionPending(dir, a.Key(), collector.ActionPhaseBegin, "node1")
	assert.Error(t, err)
	_, err = NewInstanceActionPending(dir, a.Key(), collector.ActionPhaseEnd, "node1")
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestWarnBackoff(t *testing.T) {
	var b warnBackoff
	now := time.Now()
	assert.True(t, b.due(now), "the first warning is due at once")

	for d := warnBackoffMin; d < warnBackoffMax; d *= 2 {
		b.arm(now)
		assert.Equal(t, d, b.interval)
		assert.False(t, b.due(now.Add(d-time.Millisecond)))
		assert.True(t, b.due(now.Add(d)))
		now = now.Add(d)
	}
	for range 20 {
		b.arm(now)
	}
	assert.Equal(t, warnBackoffMax, b.interval, "the interval stops doubling at the max")

	b.reset()
	assert.True(t, b.due(now))
	b.arm(now)
	assert.Equal(t, warnBackoffMin, b.interval, "a reset starts over")
}

// TestOnActionSendResultsCountsTheFailures pins that a failed send stays
// queued and counted for the paced warning, and that the count clears
// when the collector accepts again.
func TestOnActionSendResultsCountsTheFailures(t *testing.T) {
	tr, _ := newTestT(t)
	tr.isSpeaker = true
	a1, a2 := newTestCollectorAction(), newTestCollectorAction()
	m1, m2 := pendingMsg(a1, "node2"), pendingMsg(a2, "node2")
	tr.onInstanceActionPending(m1)
	tr.onInstanceActionPending(m2)
	down := errors.New("PUT /api/instance/action unexpected status code: wanted 202 got 502")

	tr.onActionSendResults([]actionSendResult{
		{actionSendJob: actionSendJob{key: a1.Key(), msg: m1}, err: down},
		{actionSendJob: actionSendJob{key: a2.Key(), msg: m2}, err: down},
	})
	assert.Len(t, tr.actionFailed, 2)
	assert.Len(t, tr.actionToSend, 2, "the failed sends stay queued")
	assert.Equal(t, warnBackoffMin, tr.actionFailure.backoff.interval, "warned at once, next in 10s")
	assert.Equal(t, down, tr.actionFailLastErr)

	// failing again before the interval does not warn again
	tr.onActionSendResults([]actionSendResult{
		{actionSendJob: actionSendJob{key: a1.Key(), msg: m1}, err: down},
	})
	assert.Equal(t, warnBackoffMin, tr.actionFailure.backoff.interval)

	// once due, it warns again and the interval doubles
	tr.actionFailure.backoff.next = time.Now()
	tr.onActionSendResults([]actionSendResult{
		{actionSendJob: actionSendJob{key: a1.Key(), msg: m1}, err: down},
	})
	assert.Equal(t, 2*warnBackoffMin, tr.actionFailure.backoff.interval)

	// one accepted leaves one failing
	tr.onActionSendResults([]actionSendResult{
		{actionSendJob: actionSendJob{key: a1.Key(), msg: m1, uuid: "u1"}, done: true},
	})
	assert.Len(t, tr.actionFailed, 1)
	assert.NotZero(t, tr.actionFailure.backoff.interval)

	// none failing resets the pacing
	tr.onActionSendResults([]actionSendResult{
		{actionSendJob: actionSendJob{key: a2.Key(), msg: m2, uuid: "u2"}, done: true},
	})
	assert.Empty(t, tr.actionFailed)
	assert.Zero(t, tr.actionFailure.backoff.interval)
	assert.Nil(t, tr.actionFailLastErr)
}

// TestSendActionsStopsAtTheFirstFailure pins that a collector down costs
// one send, and one log read, per batch, not one per action queued.
func TestSendActionsStopsAtTheFirstFailure(t *testing.T) {
	tr, _ := newTestT(t)
	tr.isSpeaker = true
	tr.ctx = context.Background()
	var logReads int
	tr.actionReadLog = func(context.Context, string, collector.Action) ([]map[string]any, error) {
		logReads++
		return nil, nil
	}
	down := true
	f := &fakeRequester{respond: func(*http.Request) *http.Response {
		if down {
			return response(http.StatusBadGateway, ``)
		}
		return response(http.StatusAccepted, `null`)
	}}
	tr.client = f
	for range 3 {
		tr.onInstanceActionPending(pendingMsg(endedAction(newTestCollectorAction(), "ok"), "node2"))
	}

	runBatch := func() {
		tr.sendActions()
		select {
		case results := <-tr.actionSendResultC:
			tr.onActionSendResults(results)
		case <-time.After(5 * time.Second):
			t.Fatal("no send result")
		}
	}

	runBatch()
	assert.Len(t, f.requests, 1, "the batch stops at the first failure")
	assert.Equal(t, 1, logReads)
	assert.Len(t, tr.actionToSend, 3, "all stay queued")
	assert.Len(t, tr.actionFailed, 1, "only the one tried is counted failing")

	down = false
	runBatch()
	assert.Len(t, f.requests, 4, "a collector back takes the whole batch")
	assert.Empty(t, tr.actionToSend)
	assert.Empty(t, tr.actionFailed)
}

// TestNextActionJobsPutsTheBeginsFirst pins that a batch takes the begins
// queued before the ends, the queue holding more than a batch.
func TestNextActionJobsPutsTheBeginsFirst(t *testing.T) {
	tr, _ := newTestT(t)
	tr.isSpeaker = true
	for range actionSendMax + 2 {
		tr.onInstanceActionPending(pendingMsg(endedAction(newTestCollectorAction(), "ok"), "node2"))
	}
	for range 3 {
		tr.onInstanceActionPending(pendingMsg(newTestCollectorAction(), "node2"))
	}
	jobs := tr.nextActionJobs()
	require.Len(t, jobs, actionSendMax)
	for i, job := range jobs {
		want := collector.ActionPhaseEnd
		if i < 3 {
			want = collector.ActionPhaseBegin
		}
		assert.Equal(t, want, job.msg.Phase, "job %d", i)
	}
}
