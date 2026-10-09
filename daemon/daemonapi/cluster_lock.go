package daemonapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/locktable"
)

// PostClusterLock acquires a cluster lock.
//
// Every lock is granted by the node speaking for the cluster, so that two
// asked at once on two nodes are weighed against each other. A node that is
// not it hands the request over, and records the lock its client was granted:
// it is what a node taking the speaking over rebuilds its table from.
//
// A request handed over is never handed over again. The node it was handed
// to was the speaker for the node handing it, and two nodes disagreeing on
// who speaks is a moment to answer "ask again", not a ring to send it around.
func (a *DaemonAPI) PostClusterLock(ctx echo.Context) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	var payload api.PostClusterLock
	if err := ctx.Bind(&payload); err != nil {
		return JSONProblem(ctx, http.StatusBadRequest, "Invalid Body", err.Error())
	}
	if payload.Name == "" {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid Body", "a lock needs a name")
	}
	req := locktable.Request{Name: payload.Name, Node: a.localhost}
	if payload.Holder != nil {
		req.Holder = *payload.Holder
	}
	var err error
	if req.Lease, err = parseOptionalDuration(payload.Lease); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid Body", "lease: %s", err)
	}
	if req.Wait, err = parseOptionalDuration(payload.Wait); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid Body", "wait: %s", err)
	}
	handedOver := payload.Node != nil && *payload.Node != ""
	if handedOver {
		req.Node = *payload.Node
	}

	speaker := speakerNode()
	if speaker != "" && speaker != a.localhost {
		if handedOver {
			return JSONProblemf(ctx, http.StatusServiceUnavailable, "Not the speaker", "%s speaks for the cluster: ask again", speaker)
		}
		return a.handOverClusterLock(ctx, speaker, payload)
	}

	if err := a.rebuildLockTable(ctx); err != nil {
		return JSONProblemf(ctx, http.StatusServiceUnavailable, "Lock table", "%s", err)
	}
	lock, err := locktable.SpeakerTable.Acquire(ctx.Request().Context(), req)
	var errHeld locktable.ErrHeld
	switch {
	case errors.As(err, &errHeld):
		return JSONProblemf(ctx, http.StatusConflict, "Lock held", "%s", err)
	case err != nil:
		return JSONProblemf(ctx, http.StatusServiceUnavailable, "Lock", "%s", err)
	}
	if !handedOver {
		locktable.LocalHeld.Add(lock)
	}
	LogHandler(ctx, "PostClusterLock").Infof("cluster lock %s granted to %s on %s until %s", lock.Name, lock.Holder, lock.Node, lock.ExpiresAt.Format(time.RFC3339))
	return ctx.JSON(http.StatusOK, lockToAPI(lock))
}

// handOverClusterLock asks the node speaking for the cluster for the lock,
// and records it held by a client of this node when it is granted.
func (a *DaemonAPI) handOverClusterLock(ctx echo.Context, speaker string, payload api.PostClusterLock) error {
	// The node asks, as it records what its clients hold on their behalf.
	c, err := newPeerClient(speaker)
	if err != nil {
		return JSONProblemf(ctx, http.StatusServiceUnavailable, "Lock", "ask %s: %s", speaker, err)
	}
	payload.Node = &a.localhost
	resp, err := c.PostClusterLockWithResponse(ctx.Request().Context(), payload)
	if err != nil {
		return JSONProblemf(ctx, http.StatusServiceUnavailable, "Lock", "ask %s: %s", speaker, err)
	}
	if resp.JSON200 == nil {
		return ctx.Blob(resp.StatusCode(), resp.HTTPResponse.Header.Get("Content-Type"), resp.Body)
	}
	locktable.LocalHeld.Add(lockFromAPI(*resp.JSON200))
	return ctx.JSON(http.StatusOK, resp.JSON200)
}

// DeleteClusterLock releases a cluster lock.
func (a *DaemonAPI) DeleteClusterLock(ctx echo.Context, params api.DeleteClusterLockParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	handedOver := params.Node != nil && *params.Node != ""
	if !handedOver {
		// Whatever the speaker answers, the client has let go of it.
		locktable.LocalHeld.Remove(params.Name, params.Id)
	}
	speaker := speakerNode()
	if speaker != "" && speaker != a.localhost {
		if handedOver {
			return JSONProblemf(ctx, http.StatusServiceUnavailable, "Not the speaker", "%s speaks for the cluster: ask again", speaker)
		}
		c, err := newPeerClient(speaker)
		if err != nil {
			return JSONProblemf(ctx, http.StatusServiceUnavailable, "Release", "ask %s: %s", speaker, err)
		}
		params.Node = &a.localhost
		resp, err := c.DeleteClusterLockWithResponse(ctx.Request().Context(), &params)
		if err != nil {
			return JSONProblemf(ctx, http.StatusServiceUnavailable, "Release", "ask %s: %s", speaker, err)
		}
		if resp.StatusCode() == http.StatusNoContent {
			return ctx.NoContent(http.StatusNoContent)
		}
		return ctx.Blob(resp.StatusCode(), resp.HTTPResponse.Header.Get("Content-Type"), resp.Body)
	}
	if err := a.rebuildLockTable(ctx); err != nil {
		return JSONProblemf(ctx, http.StatusServiceUnavailable, "Lock table", "%s", err)
	}
	if !locktable.SpeakerTable.Release(params.Name, params.Id) {
		return JSONProblemf(ctx, http.StatusNotFound, "Not held", "lock %s is not held under %s: it was released, or its lease ended", params.Name, params.Id)
	}
	LogHandler(ctx, "DeleteClusterLock").Infof("cluster lock %s released", params.Name)
	return ctx.NoContent(http.StatusNoContent)
}

// GetClusterLocks returns the cluster locks held, as the node speaking for
// the cluster granted them.
func (a *DaemonAPI) GetClusterLocks(ctx echo.Context) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	speaker := speakerNode()
	if speaker != "" && speaker != a.localhost {
		return a.proxy(ctx, speaker, func(c *client.T) (*http.Response, error) {
			return c.GetClusterLocks(ctx.Request().Context())
		})
	}
	if err := a.rebuildLockTable(ctx); err != nil {
		return JSONProblemf(ctx, http.StatusServiceUnavailable, "Lock table", "%s", err)
	}
	return ctx.JSON(http.StatusOK, lockListToAPI(locktable.SpeakerTable.List()))
}

// GetNodeLocks returns the cluster locks the clients of a node hold.
func (a *DaemonAPI) GetNodeLocks(ctx echo.Context, nodename api.InPathNodeName) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	if nodename != a.localhost {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetNodeLocks(ctx.Request().Context(), nodename)
		})
	}
	return ctx.JSON(http.StatusOK, lockListToAPI(locktable.LocalHeld.List()))
}

func parseOptionalDuration(s *string) (time.Duration, error) {
	if s == nil || *s == "" {
		return 0, nil
	}
	return time.ParseDuration(*s)
}
