package daemonapi

import (
	"encoding/json"
	"net/http"

	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/daemonlogctx"
)

func (a *DaemonApi) GetObjectSelector(w http.ResponseWriter, r *http.Request, params api.GetObjectSelectorParams) {
	log := daemonlogctx.Logger(r.Context()).With().Str("func", "GetObjectSelector").Logger()
	log.Debug().Msg("starting")
	paths := object.StatusData.GetPaths()
	selection := objectselector.NewSelection(
		params.Selector,
		objectselector.SelectionWithInstalled(paths),
		objectselector.SelectionWithLocal(true),
	)
	matchedPaths, err := selection.Expand()
	if err != nil {
		log.Error().Err(err).Msg("expand selection")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	result := api.ObjectSelector{}
	for _, v := range matchedPaths {
		result = append(result, v.String())
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
