package objecthandler

import (
	"github.com/go-chi/chi/v5"
)

func Router() *chi.Mux {
	r := chi.NewRouter()
	r.Get("/config", GetConfig)
	r.Get("/config_file", GetConfigFile)
	r.Post("/monitor", PostMonitor)
	r.Post("/status", PostStatus)
	r.Get("/selector", GetSelector)
	return r
}
