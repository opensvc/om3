package daemonapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonApi) PostNodeFile(w http.ResponseWriter, r *http.Request, params api.PostNodeFileParams) {
	payload := api.ObjectFile{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.Contains(params.Name, "..") || strings.HasPrefix(params.Name, "/") {
		sendError(w, http.StatusBadRequest, "The 'name' parameter must be a basename.")
		return
	}
	var cf string
	switch params.Kind {
	case "drbd":
		cf = fmt.Sprintf("/etc/drbd.d/%s.res", params.Name)
	default:
		sendError(w, http.StatusBadRequest, "Unknown 'kind' parameter value.")
		return
	}
	if err := os.WriteFile(cf, payload.Data, os.ModePerm); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}
