//go:generate oapi-codegen --config=codegen_server.yaml ./api.yaml
//go:generate oapi-codegen --config=codegen_type.yaml ./api.yaml

package daemonapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"opensvc.com/opensvc/daemon/daemonlogctx"
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

func sendErrorf(w http.ResponseWriter, code int, format string, a ...any) {
	msg := fmt.Sprintf(format, a)
	err := Error{
		Code:    int32(code),
		Message: msg,
	}
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(err)
}

func getLogger(r *http.Request, name string) zerolog.Logger {
	return daemonlogctx.Logger(r.Context()).With().Str("func", name).Logger()
}
