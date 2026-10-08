package actionrouter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/session"
	"github.com/opensvc/om3/v3/util/hostname"
)

// The daemon caps how long it holds one request, so a wait longer than the
// cap is that request asked again: the hold is what is left of the wait, up
// to the cap, and never the whole of a longer wait.
func TestTheHoldIsWhatIsLeftOfTheWaitUpToTheCap(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*maxHold)
	defer cancel()
	assert.Equal(t, maxHold, holdDuration(ctx, maxHold),
		"a wait longer than the cap is held for the cap, and asked again")

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	hold := holdDuration(ctx, maxHold)
	assert.Less(t, hold, 10*time.Second, "a second is kept for the round trip")
	assert.Greater(t, hold, 8*time.Second)

	assert.Equal(t, maxHold, holdDuration(context.Background(), maxHold),
		"a caller that set no deadline holds for the cap, and asks again")
}

// Whether to ask again is the caller's deadline, not the daemon's cap: a wait
// that has time left is not over because the hold expired.
func TestAskingAgainFollowsTheCallerDeadline(t *testing.T) {
	assert.True(t, hasTimeLeft(context.Background()),
		"a caller that set no deadline waits for as long as it takes")

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	assert.True(t, hasTimeLeft(ctx))

	ctx, cancel = context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond)
	assert.False(t, hasTimeLeft(ctx), "an expired deadline ends the wait")

	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	assert.False(t, hasTimeLeft(ctx))
}

// fakeDaemon answers the orchestration requests of the nodes it is given, and
// records what was asked: the node, and the hold asked for.
type fakeDaemon struct {
	mu      sync.Mutex
	asked   []string
	holds   []time.Duration
	answers map[string]func(w http.ResponseWriter)
}

func (f *fakeDaemon) client(t *testing.T) *client.T {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// /api/node/name/<node>/daemon/orchestration/id/<id>
		parts := strings.Split(r.URL.Path, "/")
		node := parts[4]
		hold, _ := time.ParseDuration(r.URL.Query().Get("wait"))
		f.mu.Lock()
		f.asked = append(f.asked, node)
		f.holds = append(f.holds, hold)
		f.mu.Unlock()
		answer, ok := f.answers[node]
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		answer(w)
	}))
	t.Cleanup(srv.Close)
	c, err := client.New(client.WithURL(srv.URL), client.WithInsecureSkipVerify(true), client.WithBearer("tk"))
	require.NoError(t, err)
	return c
}

func answerState(state string, errS string) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		item := api.OrchestrationItem{State: state}
		if errS != "" {
			item.Error = &errS
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(item)
	}
}

func answerGone(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusGone)
	_ = json.NewEncoder(w).Encode(api.Problem{Status: http.StatusGone, Title: "Orchestration no longer known"})
}

// The node that accepted the orchestration is asked first, as it knows the id
// at once, then the daemon the client talks to, until it has seen the end
// too: an id it never saw there is no error.
func TestWaitNodeOrchestrationAsksTheAcceptingNodeThenTheLocalDaemon(t *testing.T) {
	f := &fakeDaemon{answers: map[string]func(http.ResponseWriter){
		"n3":               answerState(string(session.StateSucceeded), ""),
		api.AliasLocalhost: answerGone,
	}}
	c := f.client(t)

	require.NoError(t, WaitNodeOrchestration(context.Background(), c, "n3", uuid.New()))
	assert.Equal(t, []string{"n3", api.AliasLocalhost}, f.asked)
}

// How it went is what the accepting node says: a refusal is answered there,
// and the local daemon, which never heard of it, is not asked.
func TestWaitNodeOrchestrationReportsTheAcceptingNodeOutcome(t *testing.T) {
	f := &fakeDaemon{answers: map[string]func(http.ResponseWriter){
		"n3": answerState(string(session.StateRefused), "no changes"),
	}}
	c := f.client(t)

	err := WaitNodeOrchestration(context.Background(), c, "n3", uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no changes")
	assert.Equal(t, []string{"n3"}, f.asked)
}

// A request to a peer is forwarded by the daemon with a client that gives up
// after its timeout: it is held for less, and asked again.
func TestWaitNodeOrchestrationHoldsAForwardedRequestLessThanTheForwardTimeout(t *testing.T) {
	f := &fakeDaemon{answers: map[string]func(http.ResponseWriter){
		"n3":               answerState(string(session.StateSucceeded), ""),
		api.AliasLocalhost: answerState(string(session.StateSucceeded), ""),
	}}
	c := f.client(t)

	require.NoError(t, WaitNodeOrchestration(context.Background(), c, "n3", uuid.New()))
	require.Len(t, f.holds, 2)
	assert.Less(t, f.holds[0], client.DefaultClientTimeout, "the forwarded request")
	assert.Equal(t, maxHold, f.holds[1], "the local daemon holds as long as it may")
}

// A node action on the node the client runs on is waited for on the local
// daemon only, which accepted it.
func TestWaitNodeOrchestrationOfTheLocalNode(t *testing.T) {
	f := &fakeDaemon{answers: map[string]func(http.ResponseWriter){
		api.AliasLocalhost: answerState(string(session.StateSucceeded), ""),
	}}
	c := f.client(t)

	require.NoError(t, WaitNodeOrchestration(context.Background(), c, hostname.Hostname(), uuid.New()))
	assert.Equal(t, []string{api.AliasLocalhost}, f.asked)
}
