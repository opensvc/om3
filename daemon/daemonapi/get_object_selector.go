package daemonapi

import (
	"encoding/json"
	"net/http"

	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/daemon/daemonlogctx"
)

func (a *DaemonApi) GetObjectSelector(w http.ResponseWriter, r *http.Request, params GetObjectSelectorParams) {
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
	result := ObjectSelector{}
	for _, v := range matchedPaths {
		result = append(result, v.String())
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}
