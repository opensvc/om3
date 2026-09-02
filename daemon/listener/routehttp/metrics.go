package routehttp

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/opensvc/om3/v3/util/metricsreg"
)

// The echo middleware breaks every one of its four metrics down by url,
// and two of them are histograms, so a single route reached with one
// method and one status code is 36 series. On a cluster whose nodes talk
// to each other over the api that is 1025 of the 1100 series /metrics
// answered with, and it grows with the number of routes exercised, not
// with anything an operator asked to watch. It goes to /metrics/api.
//
// What is left here is the request rate by code and method, and the
// latency of all requests taken together: enough to alert on, and to
// tell that finding out which route is worth a scrape of the detail.
var (
	// Hints, on the default registry.

	requestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "opensvc_api_requests_total",
			Help: "The total number of http requests served, by status code and method (per route at /metrics/api)",
		},
		[]string{"code", "method"})

	requestDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "opensvc_api_request_duration_seconds",
			Help:    "The latency of the http requests served, all routes taken together (per route at /metrics/api)",
			Buckets: prometheus.DefBuckets,
		})

	// Detail, served at /metrics/api. The route_ prefix says what the
	// aggregates above are broken down by, as the scheduler's
	// object_runs_total does for its runs_total.
	mwProm = echoprometheus.NewMiddlewareWithConfig(echoprometheus.MiddlewareConfig{
		Subsystem:  "opensvc_api",
		Registerer: metricsreg.API,
		CounterOptsFunc: func(opts prometheus.CounterOpts) prometheus.CounterOpts {
			opts.Name = "route_" + opts.Name
			return opts
		},
		HistogramOptsFunc: func(opts prometheus.HistogramOpts) prometheus.HistogramOpts {
			opts.Name = "route_" + opts.Name
			return opts
		},
		// Without this, a request no route matches is labelled with the
		// path it asked for, so anything scanning the listener writes
		// series nothing ever removes.
		DoNotUseRequestPathFor404: true,
	})
)

// mwPromHint feeds the aggregates the default registry serves.
//
// It resolves the status code the way echoprometheus does, so that a
// count here and the counts at /metrics/api are the same requests seen
// twice, not two different opinions of what happened.
func mwPromHint(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		start := time.Now()
		err := next(c)
		requestDuration.Observe(time.Since(start).Seconds())
		requestsTotal.WithLabelValues(strconv.Itoa(statusCode(c, err)), c.Request().Method).Inc()
		return err
	}
}

func statusCode(c echo.Context, err error) int {
	status := c.Response().Status
	if err != nil {
		var httpError *echo.HTTPError
		if errors.As(err, &httpError) {
			status = httpError.Code
		}
		if status == 0 || status == http.StatusOK {
			status = http.StatusInternalServerError
		}
	}
	return status
}
