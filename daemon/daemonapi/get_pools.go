package daemonapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/pool"
	"github.com/opensvc/om3/daemon/api"
)

// GetDaemonDNSDump returns the DNS zone content.
func (a *DaemonApi) GetPools(w http.ResponseWriter, r *http.Request, params api.GetPoolsParams) {
	n, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		WriteProblem(w, http.StatusInternalServerError, "Failed to allocate a new object.Node", fmt.Sprint(err))
		return
	}
	var l pool.StatusList
	if params.Name != nil {
		l = n.ShowPoolsByName(*params.Name)
	} else {
		l = n.ShowPools()
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(l)
	w.WriteHeader(http.StatusOK)
}
