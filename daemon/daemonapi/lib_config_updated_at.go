package daemonapi

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/file"
)

// assertConfigUpdatedAt reports whether an action may run on the configuration
// this node holds for an object.
//
// A caller that wrote a configuration and then acts on the instances races the
// write: it is acknowledged by the node that received it, and reaches the peer
// nodes a moment later. Naming the timestamp the write answered with turns
// that race into an answer, so the caller learns that this node is still on
// the configuration it meant to replace instead of watching the action run
// with the old values.
//
// The file is read rather than the configuration the daemon caches, because
// the file is what the action is about to read, and the cache trails it by the
// time it takes to notice the change.
//
// It refuses rather than waits. These actions are accepted and executed
// asynchronously, so waiting here would hold the request open on a node that
// may never receive the configuration, and would move the race into the
// execution rather than end it.
func assertConfigUpdatedAt(ctx echo.Context, p naming.Path, requested *time.Time) (bool, error) {
	if requested == nil {
		return true, nil
	}
	mtime := file.ModTime(p.ConfigFile())
	if mtime.IsZero() {
		return false, JSONProblemf(ctx, http.StatusConflict, "Config too old",
			"%s has no configuration on this node, and the action requires the one of %s",
			p, requested.Format(time.RFC3339Nano))
	}
	if mtime.Before(*requested) {
		return false, JSONProblemf(ctx, http.StatusConflict, "Config too old",
			"%s is configured here as of %s, and the action requires the configuration of %s, which has not landed yet",
			p, mtime.Format(time.RFC3339Nano), requested.Format(time.RFC3339Nano))
	}
	return true, nil
}
