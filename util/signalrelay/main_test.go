package signalrelay

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The first signal is sent to the subscribers, those subscribing after it
// included, and does not end the process.
func TestTheFirstSignalIsRelayed(t *testing.T) {
	ctx, stop := WithSignals(context.Background(), syscall.SIGUSR1)
	defer stop()
	c, unsubscribe := Subscribe(ctx)
	defer unsubscribe()
	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGUSR1))
	select {
	case sig := <-c:
		assert.Equal(t, syscall.SIGUSR1, sig)
	case <-time.After(5 * time.Second):
		t.Fatal("no signal relayed")
	}
	late, unsubscribeLate := Subscribe(ctx)
	defer unsubscribeLate()
	select {
	case sig := <-late:
		assert.Equal(t, syscall.SIGUSR1, sig, "a subscriber after the signal gets it")
	default:
		t.Fatal("a subscriber after the signal does not get it")
	}
	assert.Equal(t, syscall.SIGUSR1, Received(ctx))
}

// A context with no relay relays nothing.
func TestNoRelay(t *testing.T) {
	c, unsubscribe := Subscribe(context.Background())
	defer unsubscribe()
	assert.Nil(t, c)
	assert.Nil(t, Received(context.Background()))
}
