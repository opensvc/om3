package daemonapi

import (
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/rbac"
	"github.com/opensvc/om3/util/drbd"
)

type (
	pendingDRBDAllocationsMap struct {
		sync.Mutex
		m map[uuid.UUID]api.DRBDAllocation
	}
)

var (
	pendingDRBDAllocations *pendingDRBDAllocationsMap
)

func newPendingDRBDAllocationsMap() *pendingDRBDAllocationsMap {
	t := pendingDRBDAllocationsMap{
		m: make(map[uuid.UUID]api.DRBDAllocation),
	}
	return &t
}

func (t *pendingDRBDAllocationsMap) get(id uuid.UUID) (api.DRBDAllocation, bool) {
	a, ok := t.m[id]
	return a, ok
}

func (t *pendingDRBDAllocationsMap) minors() []int {
	l := make([]int, len(t.m))
	i := 0
	for _, a := range t.m {
		l[i] = a.Minor
		i += 1
	}
	return l
}

func (t *pendingDRBDAllocationsMap) ports() []int {
	l := make([]int, len(t.m))
	i := 0
	for _, a := range t.m {
		l[i] = a.Port
		i += 1
	}
	return l
}

func (t *pendingDRBDAllocationsMap) expire() {
	now := time.Now()
	for id, a := range t.m {
		if a.ExpiredAt.After(now) {
			delete(t.m, id)
		}
	}
}

func (t *pendingDRBDAllocationsMap) add(a api.DRBDAllocation) {
	t.m[a.Id] = a
}

func init() {
	pendingDRBDAllocations = newPendingDRBDAllocationsMap()
}

func (a *DaemonApi) GetNodeDRBDAllocation(ctx echo.Context, nodename string) error {
	if v, err := assertGrant(ctx, rbac.GrantRoot); !v {
		return err
	}
	if a.localhost == nodename {
		return a.getLocalNodeDRBDAllocation(ctx)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s is not a cluster node", nodename)
	} else {
		return a.getPeerDRBDAllocation(ctx, nodename)
	}
}

func (a *DaemonApi) getPeerDRBDAllocation(ctx echo.Context, nodename string) error {
	c, err := newProxyClient(ctx, nodename)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
	}
	if resp, err := c.GetNodeDRBDAllocationWithResponse(ctx.Request().Context(), nodename); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
	} else if len(resp.Body) > 0 {
		return ctx.JSONBlob(resp.StatusCode(), resp.Body)
	}
	return nil
}

func (a *DaemonApi) getLocalNodeDRBDAllocation(ctx echo.Context) error {
	log := LogHandler(ctx, "GetNodeDRBDAllocation")
	log.Debugf("starting")

	pendingDRBDAllocations.Lock()
	defer pendingDRBDAllocations.Unlock()
	pendingDRBDAllocations.expire()

	resp := api.DRBDAllocation{
		Id:        uuid.New(),
		ExpiredAt: time.Now().Add(5 * time.Second),
	}

	digest, err := drbd.GetDigest()
	if err != nil {
		detail := "get drbd dump digest: %s"
		log.Errorf(detail, err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "Get Node DRBD allocation", detail, err)
	}

	if minor, err := digest.FreeMinor(pendingDRBDAllocations.minors()); err != nil {
		detail := "get free minor from drbd dump digest: %s"
		log.Errorf(detail, err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "Get Node DRBD allocation", detail, err)
	} else {
		resp.Minor = minor
	}

	if port, err := digest.FreePort(pendingDRBDAllocations.ports()); err != nil {
		detail := "get free port from drbd dump digest: %s"
		log.Errorf(detail, err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "Get Node DRBD allocation", detail, err)
	} else {
		resp.Port = port
	}

	pendingDRBDAllocations.add(resp)
	return ctx.JSON(http.StatusOK, resp)
}
