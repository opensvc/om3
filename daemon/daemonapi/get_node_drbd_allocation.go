package daemonapi

import (
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/daemon/api"
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

func (a *DaemonApi) GetNodeDRBDAllocation(ctx echo.Context) error {
	log := LogHandler(ctx, "GetNodeDRBDAllocation")
	log.Debug().Msg("starting")

	pendingDRBDAllocations.Lock()
	defer pendingDRBDAllocations.Unlock()
	pendingDRBDAllocations.expire()

	resp := api.DRBDAllocation{
		Id:        uuid.New(),
		ExpiredAt: time.Now().Add(5 * time.Second),
	}

	digest, err := drbd.GetDigest()
	if err != nil {
		detail := "get drbd dump digest"
		log.Error().Err(err).Msgf(detail)
		return JSONProblemf(ctx, http.StatusInternalServerError, "Get Node DRBD allocation", detail)
	}

	if minor, err := digest.FreeMinor(pendingDRBDAllocations.minors()); err != nil {
		detail := "get free minor from drbd dump digest"
		log.Error().Err(err).Msgf(detail)
		return JSONProblemf(ctx, http.StatusInternalServerError, "Get Node DRBD allocation", detail)
	} else {
		resp.Minor = minor
	}

	if port, err := digest.FreePort(pendingDRBDAllocations.ports()); err != nil {
		detail := "get free port from drbd dump digest"
		log.Error().Err(err).Msgf(detail)
		return JSONProblemf(ctx, http.StatusInternalServerError, "Get Node DRBD allocation", detail)
	} else {
		resp.Port = port
	}

	pendingDRBDAllocations.add(resp)
	return ctx.JSON(http.StatusOK, resp)
}
