// Package collector is the daemon collector main goroutine
package collector

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/collector"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
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
		bus        *pubsub.Bus
		created    map[string]time.Time

		status daemonsubsystem.Collector

		postTicker *time.Ticker

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

		// instanceConfigChange is a map of InstanceConfigUpdated populated
		// from localhost events: InstanceConfigUpdated/InstanceConfigDeleted.
		// On ticker event those updates are posted to the collector.
		instanceConfigChange map[naming.Path]*msgbus.InstanceConfigUpdated

		// isSpeaker is true when localhost NodeStatus.IsSpeaker is true
		isSpeaker bool

		subQS pubsub.QueueSizer
	}

	requester interface {
		Do(*http.Request) (*http.Response, error)
		DoRequest(method string, relPath string, body io.Reader) (*http.Response, error)
		NewRequest(method string, relPath string, body io.Reader) (*http.Request, error)
	}

	clusterDataer interface {
		ClusterData() *cluster.Data
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

	statusPost struct {
		PreviousUpdatedAt time.Time     `json:"previous_updated_at"`
		UpdatedAt         time.Time     `json:"updated_at"`
		Data              *cluster.Data `json:"data"`
	}

	// End action:
	// {
	//   "level":"error",
	//   "node":"dev2n1",
	//   "sid":"cb373a76-991a-48e1-af18-2003b29d5b2e",
	//   "obj_path":"foo014",
	//   "node":"dev2n1",
	//   "sid":"cb373a76-991a-48e1-af18-2003b29d5b2e",
	//   "argv":["./om3","foo01*","start","--local","--log=info","--caller"],
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
)

func New(ctx context.Context, subQS pubsub.QueueSizer, opts ...funcopt.O) *T {
	t := &T{
		log:         plog.NewDefaultLogger().WithPrefix("daemon: collector: ").Attr("pkg", "daemon/collector"),
		localhost:   hostname.Hostname(),
		clusterData: daemondata.FromContext(ctx),
		subQS:       subQS,
		status: daemonsubsystem.Collector{
			DaemonSubsystemStatus: daemonsubsystem.DaemonSubsystemStatus{
				ID:           "collector",
				ConfiguredAt: time.Now(),
				CreatedAt:    time.Now(),
				State:        "undef",
			},
		},
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
	if node, err := object.NewNode(); err != nil {
		t.client = nil
		return err
	} else if cli, err := node.CollectorClient(); err != nil {
		t.client = nil
		return err
	} else {
		t.client = cli
		return nil
	}
}

func (t *T) Start(ctx context.Context) error {
	errC := make(chan error)
	t.ctx, t.cancel = context.WithCancel(ctx)

	t.bus = pubsub.BusFromContext(t.ctx)

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
		errC <- nil
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
	sub := t.bus.Sub("daemon.collector", t.subQS)
	labelLocalhost := pubsub.Label{"node", t.localhost}
	sub.AddFilter(&msgbus.ClusterConfigUpdated{}, labelLocalhost)
	sub.AddFilter(&msgbus.InstanceConfigUpdated{}, labelLocalhost)
	sub.AddFilter(&msgbus.InstanceConfigDeleted{}, labelLocalhost)
	sub.AddFilter(&msgbus.InstanceStatusDeleted{})
	sub.AddFilter(&msgbus.InstanceStatusUpdated{})
	sub.AddFilter(&msgbus.NodeConfigUpdated{}, labelLocalhost)
	sub.AddFilter(&msgbus.NodeMonitorDeleted{})
	sub.AddFilter(&msgbus.NodeStatusUpdated{})
	sub.AddFilter(&msgbus.ObjectStatusUpdated{})
	sub.AddFilter(&msgbus.ObjectStatusDeleted{})
	sub.Start()
	return sub
}

func (t *T) loop() {
	// TODO: dbopensvc value, isSpeaker should enable/disable collector
	t.log.Infof("loop started")
	t.initChanges()
	sub := t.startSubscriptions()
	defer func() {
		t.status.DaemonSubsystemStatus.State = "stopped"
		t.bus.Pub(&msgbus.DaemonCollector{Node: t.localhost, Value: t.status}, pubsub.Label{"node", t.localhost})

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
	t.nodeFrozenAt = map[string]time.Time{}
	t.instanceConfigChange = make(map[naming.Path]*msgbus.InstanceConfigUpdated)

	for _, v := range object.StatusData.GetAll() {
		t.daemonStatusChange[v.Path.String()] = struct{}{}
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
}

func (t *T) dropChanges() {
	t.changes.instanceStatusUpdates = make(map[string]*msgbus.InstanceStatusUpdated)
	t.changes.instanceStatusDeletes = make(map[string]*msgbus.InstanceStatusDeleted)
	t.daemonStatusChange = make(map[string]struct{})
}
