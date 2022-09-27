package daemonapi

import (
	"encoding/json"
	"net/http"

	"opensvc.com/opensvc/core/objectselector"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/daemonlogctx"
)

func (p *DaemonApi) GetObjectSelector(w http.ResponseWriter, r *http.Request, params GetObjectSelectorParams) {
	log := daemonlogctx.Logger(r.Context()).With().Str("func", "GetObjectSelector").Logger()
	log.Debug().Msg("starting")
	daemonData := daemondata.FromContext(r.Context())
	paths := daemonData.GetServicePaths()
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
	result := ObjectSelector{}
	for _, v := range matchedPaths {
		result = append(result, v.String())
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}
