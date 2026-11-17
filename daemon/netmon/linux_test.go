//go:build linux

package netmon

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/vishvananda/netlink"

	"github.com/opensvc/om3/v3/util/plog"
)

func newTestManager(ctx context.Context) *Manager {
	return &Manager{
		ctx:           ctx,
		log:           plog.NewDefaultLogger(),
		lastPublished: make(map[string]linkPublishState),
	}
}

// A subscription ending on a receive error closes its channel: the receive
// returns for the events to be subscribed to again, rather than read the
// closed channel in a busy loop for good.
func TestReceiveReturnsOnAClosedSubscription(t *testing.T) {
	for _, closeAddr := range []bool{true, false} {
		addrUpdates := make(chan netlink.AddrUpdate)
		linkUpdates := make(chan netlink.LinkUpdate)
		if closeAddr {
			close(addrUpdates)
		} else {
			close(linkUpdates)
		}
		ended := make(chan bool, 1)
		go func() { ended <- newTestManager(context.Background()).receive(addrUpdates, linkUpdates) }()
		select {
		case v := <-ended:
			assert.True(t, v, "the subscription ended")
		case <-time.After(time.Second):
			t.Fatalf("the receive did not return on a closed channel (address channel closed: %t)", closeAddr)
		}
	}
}

// The receive stops with the daemon.
func TestReceiveReturnsOnContextDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.False(t, newTestManager(ctx).receive(make(chan netlink.AddrUpdate), make(chan netlink.LinkUpdate)))
}
