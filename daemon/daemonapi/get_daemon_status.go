package daemonapi

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/daemondata"
)

type (
	daemonRefresh struct {
		*sync.Mutex
		updated time.Time
	}
)

const (
	// daemonRefreshInterval defines the maximum duration before next DaemonRefresh
	daemonRefreshInterval = 2 * time.Second
)

var (
	subRefreshed = daemonRefresh{Mutex: &sync.Mutex{}}
)

// GetDaemonStatus returns daemon data status
//
// When sub data hIt forces refreshing of sub data every 1
func (a *DaemonApi) GetDaemonStatus(w http.ResponseWriter, r *http.Request, params api.GetDaemonStatusParams) {
	now := time.Now()
	subRefreshed.Lock()
	databus := daemondata.FromContext(r.Context())
	if now.After(subRefreshed.updated.Add(daemonRefreshInterval)) {
		if err := databus.DaemonRefresh(); err != nil {

		}
		subRefreshed.updated = now
	}
	subRefreshed.Unlock()

	status := databus.ClusterData()
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
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
	w.WriteHeader(http.StatusOK)
}
