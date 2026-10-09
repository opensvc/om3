package daemonapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/locktable"
	"github.com/opensvc/om3/v3/util/plog"
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
	lock, ok := locktable.SpeakerTable.Release(params.Name, params.Id)
	if !ok {
		return JSONProblemf(ctx, http.StatusNotFound, "Not held", "lock %s is not held under %s: it was released, or its lease ended", params.Name, params.Id)
	}
	log := LogHandler(ctx, "DeleteClusterLock")
	releasedBy := a.localhost
	if handedOver {
		releasedBy = *params.Node
	}
	if lock.Node == releasedBy {
		log.Infof("cluster lock %s released", params.Name)
	} else {
		// A lock taken back from its holder.
		log.Infof("cluster lock %s held by %s on %s released from %s", lock.Name, lock.Holder, lock.Node, releasedBy)
	}
	go a.forgetLock(log, lock)
	return ctx.NoContent(http.StatusNoContent)
}

// forgetLock has every node alive forget a lock released.
//
// A table is rebuilt from what the nodes say their clients hold and what
// they granted while they spoke. The node of the holder may still record the
// lock, when it was released from another node, and the node that granted it
// keeps its grant after it stopped speaking: either would bring the lock back
// to a table rebuilt before its lease ends, held by nobody.
//
// It is done after the answer, as the release is done whatever the peers
// say: a peer that did not hear of it brings the lock back for what is left
// of its lease at most.
func (a *DaemonAPI) forgetLock(log *plog.Logger, lock locktable.Lock) {
	locktable.LocalHeld.Remove(lock.Name, lock.ID)
	locktable.SpeakerTable.Forget(lock.Name, lock.ID)
	for _, nodename := range clusternode.Get() {
		if nodename == a.localhost || node.StatusData.GetByNode(nodename) == nil {
			continue
		}
		if err := forgetNodeLock(nodename, lock); err != nil {
			log.Warnf("cluster lock %s: %s may bring it back until %s: %s", lock.Name, nodename, lock.ExpiresAt.Format(time.RFC3339), err)
		}
	}
}

// forgetNodeLock has a peer forget a lock.
func forgetNodeLock(nodename string, lock locktable.Lock) error {
	c, err := newPeerClient(nodename)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := c.DeleteNodeLockWithResponse(ctx, nodename, &api.DeleteNodeLockParams{Name: lock.Name, Id: lock.ID})
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusNoContent {
		return fmt.Errorf("unexpected status code %d", resp.StatusCode())
	}
	return nil
}

// DeleteNodeLock forgets a cluster lock the clients of a node hold, or that
// the node granted while it spoke.
func (a *DaemonAPI) DeleteNodeLock(ctx echo.Context, nodename api.InPathNodeName, params api.DeleteNodeLockParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	if nodename != a.localhost {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.DeleteNodeLock(ctx.Request().Context(), nodename, &params)
		})
	}
	locktable.LocalHeld.Remove(params.Name, params.Id)
	locktable.SpeakerTable.Forget(params.Name, params.Id)
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
	// What the clients of the node hold, and what the node granted while
	// it spoke, which its clients may not have heard yet.
	locks := append(locktable.LocalHeld.List(), locktable.SpeakerTable.Granted()...)
	return ctx.JSON(http.StatusOK, lockListToAPI(locks))
}

func parseOptionalDuration(s *string) (time.Duration, error) {
	if s == nil || *s == "" {
		return 0, nil
	}
	return time.ParseDuration(*s)
}
