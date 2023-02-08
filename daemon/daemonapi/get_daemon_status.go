package daemonapi

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/opensvc/om3/daemon/daemondata"
)

type (
	subRefresh struct {
		*sync.Mutex
		updated time.Time
	}
)

const (
	// subRefreshInterval defines the maximum duration before next SubRefresh
	subRefreshInterval = 2 * time.Second
)

var (
	subRefreshed = subRefresh{Mutex: &sync.Mutex{}}
)

// GetDaemonStatus returns daemon data status
//
// When sub data hIt forces refreshing of sub data every 1
func (a *DaemonApi) GetDaemonStatus(w http.ResponseWriter, r *http.Request, params GetDaemonStatusParams) {
	now := time.Now()
	subRefreshed.Lock()
	databus := daemondata.FromContext(r.Context())
	if now.After(subRefreshed.updated.Add(subRefreshInterval)) {
		if err := databus.SubRefresh(); err != nil {

		}
		subRefreshed.updated = now
	}
	subRefreshed.Unlock()

	status := databus.GetStatus()
	if params.Selector != nil {
		status = status.WithSelector(*params.Selector)
	}
	if params.Namespace != nil {
		status = status.WithNamespace(*params.Namespace)
	}
	if params.Relatives != nil {
		// TODO: WithRelatives()
		//status = status.WithRelatives(*params.Relatives)
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(status)
}
