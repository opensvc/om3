package actionrouter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
