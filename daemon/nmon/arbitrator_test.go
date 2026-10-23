package nmon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type pubNop struct{}

func (pubNop) Pub(pubsub.Messager, ...pubsub.Label) {}

// An arbitrator slow to answer does not hold the monitor: the check runs out
// of the loop, and a request reaching the monitor meanwhile is answered. A
// check asked while one runs runs once it ends, the configuration having
// changed, and only its result is recorded.
func TestTheArbitratorCheckDoesNotHoldTheMonitor(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := &Manager{
		ctx:         ctx,
		log:         plog.NewDefaultLogger(),
		publisher:   pubNop{},
		localhost:   "n1",
		arbitrators: map[string]arbitratorConfig{"a1": {Name: "a1", URI: srv.URL, Weight: 1}},
		arbitratorC: make(chan map[string]node.ArbitratorStatus, 1),
	}

	began := time.Now()
	m.refreshArbitrators()
	assert.Less(t, time.Since(began), 100*time.Millisecond, "the check runs out of the loop")
	m.refreshArbitrators()
	assert.True(t, m.arbitratorRecheck, "a check asked while one runs is queued")

	close(release)
	m.onArbitratorsChecked(<-m.arbitratorC)
	assert.Empty(t, m.nodeStatus.Arbitrators, "the result of the outdated check is dropped")
	m.onArbitratorsChecked(<-m.arbitratorC)
	require.Contains(t, m.nodeStatus.Arbitrators, "a1")
	assert.Equal(t, status.Up, m.nodeStatus.Arbitrators["a1"].Status)
	assert.Equal(t, 1, m.arbitratorTotal())
}
