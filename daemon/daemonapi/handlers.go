package daemonapi

import (
	"fmt"
	"net/http"

	"github.com/allenai/go-swaggerui"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/daemonlogctx"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
)

type DaemonApi struct {
}

var (
	labelApi  = pubsub.Label{"origin", "api"}
	labelNode = pubsub.Label{"node", hostname.Hostname()}
)

func Register(r chi.Router, enableUi bool) {
	daemonApi := &DaemonApi{}
	if enableUi {
		r.Mount("/public/ui/", http.StripPrefix("/public/ui", swaggerui.Handler("/public/openapi")))
	}
	api.HandlerFromMux(daemonApi, r)
}

func sendError(w http.ResponseWriter, code int, message string) {
	w.WriteHeader(code)
	_, _ = w.Write([]byte(message))
}

func sendErrorf(w http.ResponseWriter, code int, format string, a ...any) {
	message := fmt.Sprintf(format, a...)
	w.WriteHeader(code)
	w.Write([]byte(message))
}

func getLogger(r *http.Request, name string) zerolog.Logger {
	return daemonlogctx.Logger(r.Context()).With().Str("func", name).Logger()
}

func setStreamHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")
}
