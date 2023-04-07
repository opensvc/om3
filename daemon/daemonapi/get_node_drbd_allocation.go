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
	pendingDrbdAllocationsMap struct {
		sync.Mutex
		m map[uuid.UUID]api.DrbdAllocation
	}
)

var (
	pendingDrbdAllocations *pendingDrbdAllocationsMap
)

func newpendingDrbdAllocationsMap() *pendingDrbdAllocationsMap {
	t := pendingDrbdAllocationsMap{
		m: make(map[uuid.UUID]api.DrbdAllocation),
	}
	return &t
}

func (t pendingDrbdAllocationsMap) get(id uuid.UUID) (api.DrbdAllocation, bool) {
	a, ok := t.m[id]
	return a, ok
}

func (t pendingDrbdAllocationsMap) minors() []int {
	l := make([]int, len(t.m))
	i := 0
	for _, a := range t.m {
		l[i] = a.Minor
		i += 1
	}
	return l
}

func (t pendingDrbdAllocationsMap) ports() []int {
	l := make([]int, len(t.m))
	i := 0
	for _, a := range t.m {
		l[i] = a.Port
		i += 1
	}
	return l
}

func (t *pendingDrbdAllocationsMap) expire() {
	now := time.Now()
	for id, a := range t.m {
		if a.ExpireAt.After(now) {
			delete(t.m, id)
		}
	}
}

func (t *pendingDrbdAllocationsMap) add(a api.DrbdAllocation) {
	t.m[a.Id] = a
}

func init() {
	pendingDrbdAllocations = newpendingDrbdAllocationsMap()
}

func (a *DaemonApi) GetNodeDrbdAllocation(w http.ResponseWriter, r *http.Request) {
	write, log := handlerhelper.GetWriteAndLog(w, r, "nodehandler.GetNodeDrbdAllocate")
	log.Debug().Msg("starting")

	pendingDrbdAllocations.Lock()
	defer pendingDrbdAllocations.Unlock()
	pendingDrbdAllocations.expire()

	resp := api.DrbdAllocation{
		Id:       uuid.New(),
		ExpireAt: time.Now().Add(5 * time.Second),
	}

	digest, err := drbd.GetDigest()
	if err != nil {
		log.Error().Err(err).Msgf("get drbd dump digest")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if minor, err := digest.FreeMinor(pendingDrbdAllocations.minors()); err != nil {
		log.Error().Err(err).Msgf("get free minor from drbd dump digest")
		w.WriteHeader(http.StatusNotFound)
		return
	} else {
		resp.Minor = minor
	}

	if port, err := digest.FreePort(pendingDrbdAllocations.ports()); err != nil {
		log.Error().Err(err).Msgf("get free port from drbd dump digest")
		w.WriteHeader(http.StatusNotFound)
		return
	} else {
		resp.Port = port
	}

	b, err := json.Marshal(resp)
	if err != nil {
		log.Error().Err(err).Msg("marshal drbd allocation")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if _, err := write(b); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	pendingDrbdAllocations.add(resp)
}
