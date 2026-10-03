package collector

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/opensvc/om3/v3/core/oc3path"
)

type (
	// instrumentedRequester is a requester counting the collector calls in
	// collectorRequestsTotal, by feed and status code.
	instrumentedRequester struct {
		requester
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
	feedDaemonChange   = "daemon_change"
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
	{oc3path.FeedDaemonChange, feedDaemonChange},
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
	}
	collectorRequestsTotal.WithLabelValues(feedOf(req.URL.Path), code).Inc()
	return resp, err
}

func (t *instrumentedRequester) NewRequestWithContext(ctx context.Context, method string, relPath string, body io.Reader) (*http.Request, error) {
	return t.requester.NewRequestWithContext(ctx, method, relPath, body)
}

// setPendingMetrics sets collectorPending from the speaker queues. A node
// not speaker holds nothing.
func (t *T) setPendingMetrics() {
	var daemonStatus, daemonChange, resInfo, objectConfig, action int
	if t.isSpeaker {
		if t.featurePostChange {
			daemonChange = len(t.changes.instanceStatusUpdates) + len(t.changes.instanceStatusDeletes)
		} else {
			daemonStatus = len(t.daemonStatusChange)
		}
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
	collectorPending.WithLabelValues(feedDaemonChange).Set(float64(daemonChange))
	collectorPending.WithLabelValues(feedResourceInfo).Set(float64(resInfo))
	collectorPending.WithLabelValues(feedObjectConfig).Set(float64(objectConfig))
	collectorPending.WithLabelValues(feedInstanceAction).Set(float64(action))
}
