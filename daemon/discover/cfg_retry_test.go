package discover

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/plog"
)

func newRetryTestManager(t *testing.T) *Manager {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &Manager{
		ctx:               ctx,
		cfgCmdC:           make(chan any, 1),
		localhost:         "n2",
		log:               plog.NewDefaultLogger(),
		cfgDeleting:       make(map[naming.Path]bool),
		cfgMTime:          make(map[string]time.Time),
		fetcherFrom:       make(map[string]string),
		fetcherCancel:     make(map[string]context.CancelFunc),
		fetcherCtx:        make(map[string]context.Context),
		fetcherNodeCancel: make(map[string]map[string]context.CancelFunc),
		fetcherUpdated:    make(map[string]time.Time),
	}
}

// startTestFetcher registers a running fetcher of the args, as
// fetchConfigFromRemote does, and returns its context.
func startTestFetcher(m *Manager, a fetchArgs) context.Context {
	s := a.path.String()
	ctx, cancel := context.WithCancel(m.ctx)
	m.fetcherCancel[s] = cancel
	m.fetcherCtx[s] = ctx
	m.fetcherFrom[s] = a.peer
	m.fetcherUpdated[s] = a.updatedAt
	m.fetcherNodeCancel[a.peer] = map[string]context.CancelFunc{s: cancel}
	return ctx
}

func shortRetryDelay(t *testing.T) {
	prev := fetchRetryDelay
	fetchRetryDelay = func(int) time.Duration { return time.Millisecond }
	t.Cleanup(func() { fetchRetryDelay = prev })
}

// A fetch that failed, as on the 401 of a token expired on its way, ends its
// fetcher and is retried: a fetcher left registered refused every later
// fetch of the object, which stayed without configuration on the node.
func TestFetchFailedIsRetried(t *testing.T) {
	shortRetryDelay(t)
	p := naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "s1"}
	m := newRetryTestManager(t)
	a := fetchArgs{path: p, peer: "n1", updatedAt: time.Now(), scope: []string{"n1", "n2"}}
	ctx := startTestFetcher(m, a)

	m.onFetchFailed(&fetchFailed{fetchArgs: a, ctx: ctx, err: errors.New("401 Unauthorized"), retry: true})
	assert.Error(t, ctx.Err(), "the fetcher is ended")
	assert.Empty(t, m.fetcherFrom[p.String()], "and not registered anymore")

	select {
	case i := <-m.cfgCmdC:
		retry, ok := i.(*fetchRetry)
		require.True(t, ok, "%#v", i)
		assert.Equal(t, 1, retry.attempt)
		assert.Equal(t, "n1", retry.peer)
	case <-time.After(time.Second):
		t.Fatal("no retry")
	}
}

// The failure of a fetcher another one replaced is not its: the running one
// is left alone, and nothing is retried.
func TestFetchFailedOfAReplacedFetcher(t *testing.T) {
	shortRetryDelay(t)
	p := naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "s1"}
	m := newRetryTestManager(t)
	a := fetchArgs{path: p, peer: "n1", updatedAt: time.Now(), scope: []string{"n2"}}
	running := startTestFetcher(m, a)
	stale, cancel := context.WithCancel(context.Background())
	cancel()

	m.onFetchFailed(&fetchFailed{fetchArgs: a, ctx: stale, err: errors.New("x"), retry: true})
	assert.NoError(t, running.Err())
	assert.Equal(t, "n1", m.fetcherFrom[p.String()])
	select {
	case i := <-m.cfgCmdC:
		t.Fatalf("unexpected retry %#v", i)
	case <-time.After(50 * time.Millisecond):
	}
}

// A fetch ending with nothing to fetch again, as a configuration not for
// this node anymore, and the last attempt, end the fetcher without retry.
func TestFetchFailedWithoutRetry(t *testing.T) {
	shortRetryDelay(t)
	p := naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "s1"}
	for name, c := range map[string]struct {
		retry   bool
		attempt int
	}{
		"not to retry":     {retry: false},
		"the last attempt": {retry: true, attempt: fetchRetryMax},
	} {
		t.Run(name, func(t *testing.T) {
			m := newRetryTestManager(t)
			a := fetchArgs{path: p, peer: "n1", updatedAt: time.Now(), scope: []string{"n2"}, attempt: c.attempt}
			ctx := startTestFetcher(m, a)
			m.onFetchFailed(&fetchFailed{fetchArgs: a, ctx: ctx, err: errors.New("x"), retry: c.retry})
			assert.Error(t, ctx.Err())
			assert.Empty(t, m.fetcherFrom[p.String()])
			select {
			case i := <-m.cfgCmdC:
				t.Fatalf("unexpected retry %#v", i)
			case <-time.After(50 * time.Millisecond):
			}
		})
	}
}

// A retry fetches nothing when the configuration is not to fetch anymore.
func TestFetchRetryNotNeeded(t *testing.T) {
	testhelper.Setup(t)
	p := naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "s1"}
	updatedAt := time.Now().Add(-time.Minute)

	t.Run("another fetch runs", func(t *testing.T) {
		m := newRetryTestManager(t)
		startTestFetcher(m, fetchArgs{path: p, peer: "n3", updatedAt: time.Now()})
		m.onFetchRetry(&fetchRetry{fetchArgs{path: p, peer: "n1", updatedAt: updatedAt, scope: []string{"n2"}}})
		assert.Equal(t, "n3", m.fetcherFrom[p.String()])
	})
	t.Run("out of scope", func(t *testing.T) {
		m := newRetryTestManager(t)
		m.onFetchRetry(&fetchRetry{fetchArgs{path: p, peer: "n1", updatedAt: updatedAt, scope: []string{"n1"}}})
		assert.Empty(t, m.fetcherFrom)
	})
	t.Run("a configuration as recent is installed", func(t *testing.T) {
		m := newRetryTestManager(t)
		cfgFile := p.ConfigFile()
		require.NoError(t, os.MkdirAll(filepath.Dir(cfgFile), 0o755))
		require.NoError(t, os.WriteFile(cfgFile, []byte("[DEFAULT]\n"), 0o644))
		require.NoError(t, os.Chtimes(cfgFile, updatedAt, updatedAt))
		m.onFetchRetry(&fetchRetry{fetchArgs{path: p, peer: "n1", updatedAt: updatedAt, scope: []string{"n2"}}})
		assert.Empty(t, m.fetcherFrom)
		require.NoError(t, os.Remove(cfgFile))
	})
	t.Run("an icfg runs a configuration as recent", func(t *testing.T) {
		m := newRetryTestManager(t)
		m.cfgMTime[p.String()] = updatedAt
		m.onFetchRetry(&fetchRetry{fetchArgs{path: p, peer: "n1", updatedAt: updatedAt, scope: []string{"n2"}}})
		assert.Empty(t, m.fetcherFrom)
	})
}
