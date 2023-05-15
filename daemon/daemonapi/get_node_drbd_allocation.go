package daemonapi

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/handlers/handlerhelper"
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

func newpendingDRBDAllocationsMap() *pendingDRBDAllocationsMap {
	t := pendingDRBDAllocationsMap{
		m: make(map[uuid.UUID]api.DRBDAllocation),
	}
	return &t
}

func (t pendingDRBDAllocationsMap) get(id uuid.UUID) (api.DRBDAllocation, bool) {
	a, ok := t.m[id]
	return a, ok
}

func (t pendingDRBDAllocationsMap) minors() []int {
	l := make([]int, len(t.m))
	i := 0
	for _, a := range t.m {
		l[i] = a.Minor
		i += 1
	}
	return l
}

func (t pendingDRBDAllocationsMap) ports() []int {
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
		if a.ExpireAt.After(now) {
			delete(t.m, id)
		}
	}
}

func (t *pendingDRBDAllocationsMap) add(a api.DRBDAllocation) {
	t.m[a.Id] = a
}

func init() {
	pendingDRBDAllocations = newpendingDRBDAllocationsMap()
}

func (a *DaemonApi) GetNodeDRBDAllocation(w http.ResponseWriter, r *http.Request) {
	_, log := handlerhelper.GetWriteAndLog(w, r, "nodehandler.GetNodeDRBDAllocate")
	log.Debug().Msg("starting")

	pendingDRBDAllocations.Lock()
	defer pendingDRBDAllocations.Unlock()
	pendingDRBDAllocations.expire()

	resp := api.DRBDAllocation{
		Id:       uuid.New(),
		ExpireAt: time.Now().Add(5 * time.Second),
	}

	digest, err := drbd.GetDigest()
	if err != nil {
		log.Error().Err(err).Msgf("get drbd dump digest")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if minor, err := digest.FreeMinor(pendingDRBDAllocations.minors()); err != nil {
		log.Error().Err(err).Msgf("get free minor from drbd dump digest")
		w.WriteHeader(http.StatusNotFound)
		return
	} else {
		resp.Minor = minor
	}

	if port, err := digest.FreePort(pendingDRBDAllocations.ports()); err != nil {
		log.Error().Err(err).Msgf("get free port from drbd dump digest")
		w.WriteHeader(http.StatusNotFound)
		return
	} else {
		resp.Port = port
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Error().Err(err).Msg("marshal drbd allocation")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	pendingDRBDAllocations.add(resp)
	w.WriteHeader(http.StatusOK)
}
