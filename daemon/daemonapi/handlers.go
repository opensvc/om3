//go:generate oapi-codegen --config=codegen_server.yaml ./api.yaml
//go:generate oapi-codegen --config=codegen_type.yaml ./api.yaml

package daemonapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type DaemonApi struct {
}

func NewDaemonApi() *DaemonApi {
	return &DaemonApi{}
}

func Register(r chi.Router) {
	daemonApi := &DaemonApi{}
	HandlerFromMux(daemonApi, r)
}

func sendError(w http.ResponseWriter, code int, message string) {
	err := Error{
		Code:    int32(code),
		Message: message,
	}
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(err)
}
