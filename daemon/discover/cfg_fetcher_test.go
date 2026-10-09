package discover

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/plog"
)

// TestAStaleFetchResultLeavesTheFetcherRunning is the regression test of a
// node keeping an older configuration of an object written twice in a row.
//
// The second write cancelled the fetcher of the first and started another,
// while the result of the first was already queued. Handling that result
// ended the fetcher of the object, which by then was the second one: its
// result was dropped, and the node kept the first configuration until the
// next write.
func TestAStaleFetchResultLeavesTheFetcherRunning(t *testing.T) {
	p, err := naming.ParsePath("test/svc/fetched")
	require.NoError(t, err)
	s := p.String()
	m := &Manager{
		localhost:         "n2",
		log:               plog.NewDefaultLogger(),
		cfgDeleting:       make(map[naming.Path]bool),
		fetcherFrom:       make(map[string]string),
		fetcherCancel:     make(map[string]context.CancelFunc),
		fetcherCtx:        make(map[string]context.Context),
		fetcherNodeCancel: make(map[string]map[string]context.CancelFunc),
		fetcherUpdated:    make(map[string]time.Time),
	}

	// The first fetcher, cancelled by the second write.
	staleCtx, staleCancel := context.WithCancel(context.Background())
	staleCancel()

	// The second fetcher, running.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.fetcherCancel[s] = cancel
	m.fetcherCtx[s] = ctx
	m.fetcherFrom[s] = "n1"
	m.fetcherUpdated[s] = time.Now()

	errC := make(chan error, 1)
	m.onRemoteConfigFetched(&msgbus.RemoteFileConfig{Path: p, Node: "n1", Ctx: staleCtx, Err: errC})
	assert.NoError(t, <-errC)
	assert.NoError(t, ctx.Err(), "the running fetcher is not cancelled by the result of the one it replaced")
	assert.Equal(t, "n1", m.fetcherFrom[s], "and stays registered")

	// Its own result ends it.
	errC = make(chan error, 1)
	cancel()
	m.onRemoteConfigFetched(&msgbus.RemoteFileConfig{Path: p, Node: "n1", Ctx: ctx, Err: errC})
	<-errC
	assert.Empty(t, m.fetcherFrom[s])
	assert.Nil(t, m.fetcherCtx[s])
}
