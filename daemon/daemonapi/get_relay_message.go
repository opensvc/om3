package daemonapi

import (
	"encoding/json"
	"net/http"

	"github.com/opensvc/om3/daemon/relay"
)

func (a *DaemonApi) GetRelayMessage(w http.ResponseWriter, r *http.Request, params GetRelayMessageParams) {
	data := RelayMessages{}
	if params.ClusterId != nil && params.Nodename != nil {
		if msg, ok := relay.Map.Load(*params.ClusterId, *params.Nodename); !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		} else {
			data.Messages = []RelayMessage{msg.(RelayMessage)}
		}
	} else {
		l := relay.Map.List()
		data.Messages = make([]RelayMessage, len(l))
		for i, a := range l {
			data.Messages[i] = a.(RelayMessage)
		}
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(data)
}
