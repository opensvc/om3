package httpmetric

import "github.com/prometheus/client_golang/prometheus"

var (
	Counter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "opensvc_api_requests_total",
			Help: "A counter for http requests with method, code and path.",
		},
		[]string{"code", "method", "path"},
	)
)

func init() {
	prometheus.MustRegister(Counter)
}
