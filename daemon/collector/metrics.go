package collector

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/opensvc/om3/v3/core/oc3path"
)

type (
	// instrumentedRequester is a requester counting the collector calls in
	// collectorRequestsTotal, by feed and status code.
	instrumentedRequester struct {
		requester

		// unauthorizedSince is the unix nano time of the first 401 of the
		// current run of 401 answers, 0 when the last answer was another.
		// The sends run off the loop, which reads it.
		unauthorizedSince atomic.Int64

		// acceptedAt is the unix nano time of the last answer not a 401.
		acceptedAt atomic.Int64
	}
)

var (
	// collectorRequestsTotal counts the collector calls of the speaker. Each
	// call carries one item, an action phase, the resource info of an
	// instance, an object config, a daemon status or ping, so its rate is
	// the rate the speaker pushes data at.
	collectorRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "opensvc",
			Subsystem: "collector",
			Name:      "requests_total",
			Help:      "The number of collector calls of the collector speaker, by feed and status code (error for a call without response)",
		}, []string{"feed", "code"})

	// collectorPending is the number of items the speaker holds for the
	// collector, by feed: what piles up while the collector is down, or not
	// configured yet.
	collectorPending = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "opensvc",
			Subsystem: "collector",
			Name:      "pending",
			Help:      "The number of items the collector speaker holds for the collector, by feed",
		}, []string{"feed"})

	// collectorActionPendingFiles is the number of the local actions whose
	// begin or end the collector did not acknowledge yet. Unlike
	// collectorPending, it is set on every node, each owning the reports of
	// its actions, so the sum over the cluster is the action backlog even
	// when there is no speaker, or a new one.
	collectorActionPendingFiles = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "opensvc",
			Subsystem: "collector",
			Name:      "action_pending_files",
			Help:      "The number of the local instance actions whose begin or end the collector did not acknowledge yet",
		})
)

const (
	feedDaemonPing     = "daemon_ping"
	feedDaemonStatus   = "daemon_status"
	feedResourceInfo   = "resource_info"
	feedObjectConfig   = "object_config"
	feedInstanceAction = "instance_action"
	feedOther          = "other"
)

// feedPaths maps the collector api paths the speaker calls to their feed
// label.
var feedPaths = []struct {
	path string
	feed string
}{
	{oc3path.FeedDaemonPing, feedDaemonPing},
	{oc3path.FeedDaemonStatus, feedDaemonStatus},
	{oc3path.FeedInstanceResinfo, feedResourceInfo},
	{oc3path.FeedObjectConfig, feedObjectConfig},
	{oc3path.FeedInstanceAction, feedInstanceAction},
}

func newInstrumentedRequester(r requester) *instrumentedRequester {
	return &instrumentedRequester{requester: r}
}

// feedOf returns the feed label of a request url path. The path holds the
// path of the collector base url before the api path, so the api path is
// matched as a suffix.
func feedOf(urlPath string) string {
	for _, e := range feedPaths {
		if strings.HasSuffix(urlPath, e.path) {
			return e.feed
		}
	}
	return feedOther
}

func (t *instrumentedRequester) Do(req *http.Request) (*http.Response, error) {
	resp, err := t.requester.Do(req)
	code := "error"
	if err == nil && resp != nil {
		code = strconv.Itoa(resp.StatusCode)
		t.observe(resp.StatusCode, time.Now())
	}
	collectorRequestsTotal.WithLabelValues(feedOf(req.URL.Path), code).Inc()
	return resp, err
}

// observe records an answer of the collector: a 401 starts or extends the run
// of credentials refused, and an accepted answer, a 2xx, ends it. Any other
// answer, as a 403 or a 5xx of a collector or a proxy failing, tells nothing
// of the credentials, nor does a request without answer, as one timing out.
func (t *instrumentedRequester) observe(code int, now time.Time) {
	switch {
	case code == http.StatusUnauthorized:
		t.unauthorizedSince.CompareAndSwap(0, now.UnixNano())
	case code >= 200 && code < 300:
		t.unauthorizedSince.Store(0)
		t.acceptedAt.Store(now.UnixNano())
	}
}

func (t *instrumentedRequester) NewRequestWithContext(ctx context.Context, method string, relPath string, body io.Reader) (*http.Request, error) {
	return t.requester.NewRequestWithContext(ctx, method, relPath, body)
}

// setPendingMetrics sets collectorPending from the speaker queues. A node
// not speaker holds nothing.
func (t *T) setPendingMetrics() {
	var daemonStatus, resInfo, objectConfig, action int
	if t.isSpeaker {
		daemonStatus = len(t.daemonStatusChange) + t.daemonStatusInFlight
		resInfo = len(t.resInfoToSend)
		objectConfig = len(t.objectConfigToSend)
		for _, e := range t.actionToSend {
			if e.begin != nil {
				action++
			}
			if e.end != nil {
				action++
			}
		}
	}
	collectorPending.WithLabelValues(feedDaemonStatus).Set(float64(daemonStatus))
	collectorPending.WithLabelValues(feedResourceInfo).Set(float64(resInfo))
	collectorPending.WithLabelValues(feedObjectConfig).Set(float64(objectConfig))
	collectorPending.WithLabelValues(feedInstanceAction).Set(float64(action))
}
