package discover

import (
	"context"
	"time"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/file"
)

type (
	// fetchArgs is what a fetch of the configuration of an object from a
	// peer is started with, and started again with when it failed.
	fetchArgs struct {
		path            naming.Path
		peer            string
		updatedAt       time.Time
		needStoppedFlag bool
		scope           []string

		// attempt is the number of the attempts that failed before.
		attempt int
	}

	// fetchFailed is what a fetcher sends when it ends without a
	// configuration to install, for the manager to end it, and to fetch
	// again when retry is set.
	//
	// A fetcher ending silently stayed registered: every later fetch of the
	// object was refused as already in progress, until a peer wrote a more
	// recent configuration, or the daemon restarted.
	fetchFailed struct {
		fetchArgs
		ctx   context.Context
		err   error
		retry bool
	}

	// fetchRetry asks the manager to fetch again, once the delay of the
	// attempt passed.
	fetchRetry struct {
		fetchArgs
	}
)

// fetchRetryMax is the number of times a fetch is retried, the delays
// between the attempts doubling from one second.
const fetchRetryMax = 5

// fetchRetryDelay is the delay before the attempt, a variable for the tests.
var fetchRetryDelay = func(attempt int) time.Duration {
	return time.Second << (attempt - 1)
}

// onFetchFailed ends the fetcher that failed, unless another one replaced
// it, and retries it after a delay.
func (t *Manager) onFetchFailed(c *fetchFailed) {
	s := c.path.String()
	if t.fetcherCtx[s] != c.ctx {
		// Cancelled, or replaced by the fetch of a more recent
		// configuration, which is the one to follow.
		return
	}
	t.cancelFetcher(s)
	if !c.retry {
		return
	}
	log := t.objectLogger(c.path)
	if c.attempt >= fetchRetryMax {
		log.Warnf("cfg: give up fetching %s from %s after %d attempts: %s", c.path, c.peer, c.attempt+1, c.err)
		return
	}
	args := c.fetchArgs
	args.attempt++
	delay := fetchRetryDelay(args.attempt)
	log.Infof("cfg: fetch %s from %s again in %s (attempt %d/%d): %s", c.path, c.peer, delay, args.attempt+1, fetchRetryMax+1, c.err)
	time.AfterFunc(delay, func() {
		select {
		case t.cfgCmdC <- &fetchRetry{fetchArgs: args}:
		case <-t.ctx.Done():
		}
	})
}

// onFetchRetry fetches again, unless the configuration is not to fetch
// anymore: another fetch runs, the node left the scope, or a configuration
// as recent is installed.
func (t *Manager) onFetchRetry(c *fetchRetry) {
	s := c.path.String()
	if _, ok := t.fetcherFrom[s]; ok {
		return
	}
	if !inList(t.localhost, c.scope) {
		return
	}
	if !c.updatedAt.After(file.ModTime(c.path.ConfigFile())) {
		return
	}
	if mtime, ok := t.cfgMTime[s]; ok && !c.updatedAt.After(mtime) {
		return
	}
	t.objectLogger(c.path).Infof("cfg: fetch config %s from %s, attempt %d/%d", c.path, c.peer, c.attempt+1, fetchRetryMax+1)
	t.fetchConfigFromRemote(c.fetchArgs)
}
