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
		WriteProblemf(w, http.StatusBadRequest, "Invalid body", "%s", err)
		return
	}
	if a, ok := pendingDrbdAllocations.get(payload.AllocationId); !ok || time.Now().After(a.ExpireAt) {
		WriteProblemf(w, http.StatusBadRequest, "Invalid body", "drbd allocation expired: %#v", a)
		return
	}
	if strings.Contains(params.Name, "..") || strings.HasPrefix(params.Name, "/") {
		WriteProblemf(w, http.StatusBadRequest, "Invalid body", "The 'name' parameter must be a basename.")
		return
	}
	cf := fmt.Sprintf("/etc/drbd.d/%s.res", params.Name)
	if err := os.WriteFile(cf, payload.Data, 0644); err != nil {
		WriteProblemf(w, http.StatusInternalServerError, "Error writing drbd res file", "%s", err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
