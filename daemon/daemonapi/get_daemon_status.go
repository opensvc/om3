package daemonapi

import (
	"encoding/json"
	"net/http"

	"opensvc.com/opensvc/daemon/daemondata"
)

func (a *DaemonApi) GetDaemonStatus(w http.ResponseWriter, r *http.Request, params GetDaemonStatusParams) {
	databus := daemondata.FromContext(r.Context())
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
