package actionrouter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/session"
)

// The daemon caps how long it holds one request, so a wait longer than the
// cap is that request asked again: the hold is what is left of the wait, up
// to the cap, and never the whole of a longer wait.
func TestTheHoldIsWhatIsLeftOfTheWaitUpToTheCap(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*maxHold)
	defer cancel()
	assert.Equal(t, maxHold, holdDuration(ctx),
		"a wait longer than the cap is held for the cap, and asked again")

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	hold := holdDuration(ctx)
	assert.Less(t, hold, 10*time.Second, "a second is kept for the round trip")
	assert.Greater(t, hold, 8*time.Second)

	assert.Equal(t, maxHold, holdDuration(context.Background()),
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

// The wait asks the node it is given about the orchestration, not the daemon
// the client talks to: the node that accepted it knows the id from the
// moment it answered it, another one only once the monitor carrying the id
// reached it.
func TestWaitAsksTheNodeItIsGiven(t *testing.T) {
	id := uuid.New()
	var asked []string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.OrchestrationItem{
			OrchestrationID: id.String(),
			Node:            "n3",
			State:           string(session.StateSucceeded),
		})
	}))
	defer srv.Close()
	c, err := client.New(client.WithURL(srv.URL), client.WithInsecureSkipVerify(true), client.WithBearer("tk"))
	require.NoError(t, err)

	require.NoError(t, WaitOrchestration(context.Background(), c, "n3", id))
	assert.Equal(t, []string{"/api/node/name/n3/daemon/orchestration/id/" + id.String()}, asked)
}
