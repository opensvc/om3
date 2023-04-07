package daemonapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonApi) PostNodeDrbdConfig(w http.ResponseWriter, r *http.Request, params api.PostNodeDrbdConfigParams) {
	payload := api.PostNodeDrbdConfigRequestBody{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}
	if a, ok := pendingDrbdAllocations.get(payload.AllocationId); !ok || time.Now().After(a.ExpireAt) {
		sendError(w, http.StatusBadRequest, fmt.Sprintf("drbd allocation expired: %#v", a))
		return
	}
	if strings.Contains(params.Name, "..") || strings.HasPrefix(params.Name, "/") {
		sendError(w, http.StatusBadRequest, "The 'name' parameter must be a basename.")
		return
	}
	cf := fmt.Sprintf("/etc/drbd.d/%s.res", params.Name)
	if err := os.WriteFile(cf, payload.Data, 0644); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}
