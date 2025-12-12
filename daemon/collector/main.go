// Package collector is the daemon collector main goroutine
package collector

import (
	"context"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/clusterdump"
	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/daemondata"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type (
	T struct {
		ctx        context.Context
		cancel     context.CancelFunc
		log        *plog.Logger
		localhost  string
		feedClient *collector.Client
		feedPinger *collector.Pinger
		client     requester
		wg         sync.WaitGroup
		publisher  pubsub.Publisher
		created    map[string]time.Time

		status daemonsubsystem.Collector

		postTicker *time.Ticker

		// postPingOrStatusAt is the timestamp of latest post daemon ping, status.
		postPingOrStatusAt time.Time

		// postPingDelay is the minimum delay to wait after postPingOrStatusAt
		// for the next post daemon ping.
		postPingDelay time.Duration

		// previousUpdatedAt is the timestamp of the last successfully data sent to
		// collector. It is used by the  collector to detect changes chain breaks.
		// When a break happens, the collector responds with a 409, and the agent
		// will post a full dataset.
		previousUpdatedAt time.Time

		// changes is used to POST /daemon/data
		changes changesData

		// instances is used to POST /daemon/ping
		instances map[string]struct{}

		// daemonStatusChange is used when we have to post data to collector:
		// if the map is empty, we POST /oc3/feed/daemon/ping
		// else we POST /oc3/feed/daemon/status with header XDaemonChange
		//
		// The map keys are:
		//   - @<nodename>: on node removed or node frozen state changed
		//   - <path>@<nodename>: on instance status updated/deleted
		//   - <path>: on object status updated/deleted
		daemonStatusChange map[string]struct{}

		// nodeFrozenAt is used to detect node frozen state changes
		nodeFrozenAt map[string]time.Time

		clusterData clusterDataer

		// featurePostChange when true, POST /oc3/feed/daemon/change instead of
		featurePostChange bool

		// objectConfigToSend is a map of path to the most recent
		// InstanceConfigUpdated populated from:
		//    1- events: InstanceConfigUpdated or InstanceConfigDeleted
		//    2- response body attr object_without_config of POST feed daemon
		//      status or ping. objectConfigToSendMinDelay is the minimum
		//      delay to wait after objectConfigSent[p].SentAt before to add
		//      the path p to the map.
		//
		// On ticker event collector POST instance config of the mapped paths
		// to the collector.
		objectConfigToSend map[naming.Path]*msgbus.InstanceConfigUpdated

		// objectConfigToSendMinDelay is the minimum interval to wait
		// after objectConfigSent[p].SentAt before adding objectConfigToSend[p].
		objectConfigToSendMinDelay time.Duration

		// objectConfigSent is a cache of known sent object config to the
		// collector.
		objectConfigSent map[naming.Path]objectConfigSent

		// isSpeaker is true when localhost NodeStatus.IsLeader is true
		isSpeaker bool

		// disable is true when collector is disabled (example ErrNodeCollectorConfig)
		disable bool

		subQS pubsub.QueueSizer

		// version is the data version
		version string

		// clusterObject is a map of cluster objects
		clusterObject map[string]struct{}

		// clusterNode is a map of cluster nodenames
		clusterNode map[string]struct{}
	}

	requester interface {
		URL() string
		Do(*http.Request) (*http.Response, error)
		NewRequestWithContext(ctx context.Context, method string, relPath string, body io.Reader) (*http.Request, error)
	}

	clusterDataer interface {
		ClusterData() *clusterdump.Data
	}

	changesData struct {
		instanceStatusUpdates map[string]*msgbus.InstanceStatusUpdated
		instanceStatusDeletes map[string]*msgbus.InstanceStatusDeleted
	}

	changesPost struct {
		PreviousUpdatedAt     time.Time                      `json:"previous_updated_at"`
		UpdatedAt             time.Time                      `json:"updated_at"`
		InstanceStatusUpdates []msgbus.InstanceStatusUpdated `json:"instance_status_update"`
		InstanceStatusDeletes []msgbus.InstanceStatusDeleted `json:"instance_status_delete"`
	}

	// End action:
	// {
	//   "level":"error",
	//   "node":"dev2n1",
	//   "sid":"cb373a76-991a-48e1-af18-2003b29d5b2e",
	//   "obj_path":"foo014",
	//   "node":"dev2n1",
	//   "sid":"cb373a76-991a-48e1-af18-2003b29d5b2e",
	//   "argv":["./om3","foo01*","instance","start","--caller"],
	//   "cwd":"/root/dev/om3",
	//   "action":"start",
	//   "origin":"user",
	//   "duration":1518.528241,
	//   "error":"abort start",
	//   "time":"2023-10-09T12:53:43.10928891+02:00",
	//   "message":"done",
	// }
	logEntry map[string]string
)

const (
	headerDaemonChange      = "XDaemonChange"
	headerPreviousUpdatedAt = "XPreviousUpdatedAt"
)

var (
	Headers = []string{
		"svcname",
		"action",
		"hostname",
		"sid",
		"version",
		"begin",
		"status_log",
		"cron",
	}
	WatchDir = filepath.Join(rawconfig.Paths.Log, "actions")

	FeedPingerInterval = time.Second * 5

	// defaultPostMaxDuration is the max duration of post request context.
	defaultPostMaxDuration = 1000 * time.Millisecond
)

func New(ctx context.Context, subQS pubsub.QueueSizer, opts ...funcopt.O) *T {
	now := time.Now()
	postPingOrStatusMinInterval := 10 * time.Second
	t := &T{
		log:           plog.NewDefaultLogger().WithPrefix("daemon: collector: ").Attr("pkg", "daemon/collector"),
		localhost:     hostname.Hostname(),
		postPingDelay: postPingOrStatusMinInterval,
		clusterData:   daemondata.FromContext(ctx),
		subQS:         subQS,
		status: daemonsubsystem.Collector{
			Status: daemonsubsystem.Status{
				CreatedAt: now,
				ID:        "collector",
				State:     "",
			},
			Url: "",
		},
		version: "3.0.0",

		objectConfigToSendMinDelay: 2 * postPingOrStatusMinInterval,
	}
	if err := funcopt.Apply(t, opts...); err != nil {
		t.log.Errorf("init: %s", err)
		return nil
	}
	return t
}

func (t *T) setNodeFeedClient() error {
	if node, err := object.NewNode(); err != nil {
		return err
	} else if client, err := node.CollectorFeedClient(); err != nil {
		return err
	} else {
		t.feedClient = client
		t.feedClient.SetLogger(t.log)
		return nil
	}
}

func (t *T) setupRequester() error {
	// TODO: pickup dbopensvc, auth, insecure from config update message
	//       to create requester from core/collector.NewRequester
	t.status.ConfiguredAt = time.Now()
	if node, err := object.NewNode(); err != nil {
		t.client = nil
		return err
	} else if cli, err := node.CollectorClient(); err != nil {
		t.client = nil
		if errors.Is(err, object.ErrNodeCollectorConfig) {
			t.disable = true
			t.status.Url = ""
			err = nil
		} else {
			// It is now enabled, clear previous disable state
			t.disable = false
			t.status.Url = "unknown"
		}
		return err
	} else {
		t.client = cli
		t.status.Url = cli.URL()
		// It is now enabled, clear previous disable state
		t.disable = false
		return nil
	}
}

func (t *T) Start(ctx context.Context) error {
	errC := make(chan error)
	t.ctx, t.cancel = context.WithCancel(ctx)

	t.publisher = pubsub.PubFromContext(t.ctx)

	t.wg.Add(1)
	go func(errC chan<- error) {
		defer t.wg.Done()
		if err := t.setNodeFeedClient(); err != nil {
			t.log.Infof("the collector routine is dormant: %s", err)
		} else {
			t.log.Infof("feeding %s", t.feedClient)
			t.feedPinger = t.feedClient.NewPinger()
			t.feedPinger.Start(t.ctx, FeedPingerInterval)
			defer t.feedPinger.Stop()
		}
		if err := t.setupRequester(); err != nil {
			t.log.Errorf("can't setup requester: %s", err)
		}
		errC <- nil
		// delay collector allows more consistent state during startup and
		// reduces state transitions: undef->speaker->speaker-candidate
		select {
		case <-time.After(5 * time.Second):
		case <-ctx.Done():
			return
		}
		t.loop()
	}(errC)

	return <-errC
}

func (t *T) Stop() error {
	t.log.Infof("stopping")
	defer t.log.Infof("stopped")
	t.cancel()
	t.wg.Wait()
	return nil
}

func (t *T) startSubscriptions() *pubsub.Subscription {
	sub := pubsub.SubFromContext(t.ctx, "daemon.collector", t.subQS)
	labelLocalhost := pubsub.Label{"node", t.localhost}

	sub.AddFilter(&msgbus.ClusterConfigUpdated{}, labelLocalhost)

	sub.AddFilter(&msgbus.InstanceConfigUpdated{})
	sub.AddFilter(&msgbus.InstanceConfigDeleted{})
	sub.AddFilter(&msgbus.InstanceStatusDeleted{})
	sub.AddFilter(&msgbus.InstanceStatusUpdated{})

	// Reminder: NodeConfigUpdated will be fired on ClusterConfigUpdated
	sub.AddFilter(&msgbus.NodeConfigUpdated{}, labelLocalhost)

	sub.AddFilter(&msgbus.NodeMonitorDeleted{})
	sub.AddFilter(&msgbus.NodeStatusUpdated{})
	sub.AddFilter(&msgbus.ObjectStatusUpdated{})
	sub.AddFilter(&msgbus.ObjectStatusDeleted{})

	sub.AddFilter(&msgbus.DaemonHeartbeatUpdated{}, pubsub.Label{"changed", "true"})

	sub.Start()
	return sub
}

func (t *T) loop() {
	// TODO: dbopensvc value, isSpeaker should enable/disable collector
	t.log.Infof("loop started")
	t.isSpeaker = !t.disable && node.StatusData.GetByNode(t.localhost).IsLeader
	t.publishOnChange(t.getState())

	t.initChanges()
	sub := t.startSubscriptions()
	defer func() {
		t.status.State = "disabled"
		t.publish()

		if err := sub.Stop(); err != nil {
			t.log.Errorf("subscription stop: %s", err)
		}
	}()

	refreshTicker := time.NewTicker(5 * time.Second)
	defer refreshTicker.Stop()

	for {
		select {
		case ev := <-sub.C:
			switch c := ev.(type) {
			case *msgbus.ClusterConfigUpdated:
				t.onClusterConfigUpdated(c)
			case *msgbus.InstanceConfigDeleted:
				t.onInstanceConfigDeleted(c)
			case *msgbus.InstanceConfigUpdated:
				t.onInstanceConfigUpdated(c)
			case *msgbus.InstanceStatusDeleted:
				t.onInstanceStatusDeleted(c)
			case *msgbus.InstanceStatusUpdated:
				t.onInstanceStatusUpdated(c)
			case *msgbus.NodeConfigUpdated:
				t.onNodeConfigUpdated(c)
			case *msgbus.DaemonHeartbeatUpdated:
				t.daemonStatusChange[c.Node] = struct{}{}
			case *msgbus.NodeMonitorDeleted:
				t.onNodeMonitorDeleted(c)
			case *msgbus.NodeStatusUpdated:
				t.onNodeStatusUpdated(c)
			case *msgbus.ObjectStatusDeleted:
				t.onObjectStatusDeleted(c)
			case *msgbus.ObjectStatusUpdated:
				t.onObjectStatusUpdated(c)
			}
		case <-refreshTicker.C:
			t.onRefreshTicker()
		case <-t.ctx.Done():
			return
		}
	}
}

func (t *T) sendBeginAction(data []string) {
	t.feedClient.Call("begin_action", Headers, data)
}

func (t *T) sendLogs(data [][]string) {
	t.feedClient.Call("res_action_batch", Headers, data)
}

func (t *T) initChanges() {
	t.previousUpdatedAt = time.Time{}
	t.instances = make(map[string]struct{})
	t.changes = changesData{
		instanceStatusUpdates: make(map[string]*msgbus.InstanceStatusUpdated),
		instanceStatusDeletes: make(map[string]*msgbus.InstanceStatusDeleted),
	}
	t.daemonStatusChange = make(map[string]struct{})
	t.clusterObject = make(map[string]struct{})
	t.clusterNode = make(map[string]struct{})
	t.nodeFrozenAt = map[string]time.Time{}
	t.objectConfigToSend = make(map[naming.Path]*msgbus.InstanceConfigUpdated)
	t.objectConfigSent = make(map[naming.Path]objectConfigSent)

	for _, v := range object.StatusData.GetAll() {
		t.daemonStatusChange[v.Path.String()] = struct{}{}
		t.clusterObject[v.Path.String()] = struct{}{}
	}

	for _, v := range instance.StatusData.GetAll() {
		i := instance.InstanceString(v.Path, v.Node)
		t.instances[i] = struct{}{}
		t.changes.instanceStatusUpdates[i] = &msgbus.InstanceStatusUpdated{
			Path:  v.Path,
			Node:  v.Node,
			Value: *v.Value,
		}

		t.daemonStatusChange[i] = struct{}{}
	}

	for _, v := range node.StatusData.GetAll() {
		t.daemonStatusChange["@"+v.Node] = struct{}{}
		t.nodeFrozenAt[v.Node] = v.Value.FrozenAt
	}

	for _, v := range instance.ConfigData.GetAll() {
		t.onInstanceConfigUpdated(&msgbus.InstanceConfigUpdated{
			Path:  v.Path,
			Node:  v.Node,
			Value: *v.Value,
		})
	}

	for _, nodename := range clusternode.Get() {
		t.clusterNode[nodename] = struct{}{}
	}
}

func (t *T) dropChanges() {
	t.changes.instanceStatusUpdates = make(map[string]*msgbus.InstanceStatusUpdated)
	t.changes.instanceStatusDeletes = make(map[string]*msgbus.InstanceStatusDeleted)
	t.daemonStatusChange = make(map[string]struct{})
}

func (t *T) publish() {
	daemonsubsystem.DataCollector.Set(t.localhost, t.status.DeepCopy())
	t.publisher.Pub(&msgbus.DaemonCollectorUpdated{Node: t.localhost, Value: *t.status.DeepCopy()}, pubsub.Label{"node", t.localhost})
}

// getState compute and return new state.
//
// possible states:
//
//	disable: node has no collector configuration
//	speaker: node is collector speaker
//	speaker-warning: node is collector speaker, but has client errors
//	speaker-candidate: node is collector speaker candidate
//	warning: node is collector speaker candidate, but has client errors
func (t *T) getState() string {
	if t.disable {
		return "disabled"
	} else if t.isSpeaker {
		if t.client != nil {
			return "speaker"
		} else {
			return "speaker-warning"
		}
	} else {
		if t.client != nil {
			return "speaker-candidate"
		} else {
			return "warning"
		}
	}
}

// publishOnChange publishes DaemonCollectorUpdated when state is changed or
// if ConfiguredAt > UpdatedAt.
// UpdatedAt is the time of last publication (updated each time publication is
// done).
func (t *T) publishOnChange(state string) {
	if state != t.status.State {
		t.log.Infof("state change %s -> %s", t.status.State, state)
		t.status.State = state
		t.status.UpdatedAt = time.Now()
		t.publish()
	} else if t.status.ConfiguredAt.After(t.status.UpdatedAt) {
		t.status.UpdatedAt = time.Now()
		t.publish()
	}
}
