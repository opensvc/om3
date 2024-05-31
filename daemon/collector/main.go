package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/collector"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
	"github.com/opensvc/om3/util/xmap"
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

		// daemonStatusChange is used to create POST /oc3/feed/daemon/status
		// header XDaemonChange
		daemonStatusChange map[string]struct{}

		clusterData clusterDataer

		// featurePostChange when true, POST /oc3/feed/daemon/change instead of
		// POST /oc3/feed/daemon/status
		featurePostChange bool
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
		previousUpdatedAt time.Time     `json:"previous_updated_at"`
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
	WatchDir              = filepath.Join(rawconfig.Paths.Log, "actions")
	SubscriptionQueueSize = 1000
	FeedPingerInterval    = time.Second * 5
)

func New(ctx context.Context, opts ...funcopt.O) *T {
	t := &T{
		log:         plog.NewDefaultLogger().WithPrefix("daemon: collector: ").Attr("pkg", "daemon/collector"),
		localhost:   hostname.Hostname(),
		clusterData: daemondata.FromContext(ctx),
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
		return err
	} else if cli, err := node.CollectorClient(); err != nil {
		return err
	} else {
		t.client = cli
		return nil
	}
}

func (t *T) Start(ctx context.Context) error {
	errC := make(chan error)
	t.ctx, t.cancel = context.WithCancel(ctx)

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
	t.bus = pubsub.BusFromContext(t.ctx)
	sub := t.bus.Sub("collector", pubsub.WithQueueSize(SubscriptionQueueSize))
	labelLocalhost := pubsub.Label{"node", t.localhost}
	sub.AddFilter(&msgbus.ClusterConfigUpdated{}, labelLocalhost)
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
	t.log.Infof("loop started")
	t.initChanges()
	sub := t.startSubscriptions()
	defer func() {
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
			err := t.sendCollectorData()
			if err != nil {
				t.log.Warnf("sendCollectorData: %s", err)
			}
		case <-t.ctx.Done():
			return
		}
	}
}

func (t *T) onClusterConfigUpdated(c *msgbus.ClusterConfigUpdated) {
	t.onConfigUpdated()
}

func (t *T) onConfigUpdated() {
	t.log.Debugf("reconfigure")
	if collector.Alive.Load() {
		t.log.Infof("disable collector clients")
		collector.Alive.Store(false)
	}
	err := t.setNodeFeedClient()
	if t.feedPinger != nil {
		t.feedPinger.Stop()
	}
	if err := t.setupRequester(); err != nil {
		t.log.Errorf("can't setup requester: %s", err)
	}
	if err != nil {
		t.log.Infof("the collector routine is dormant: %s", err)
	} else {
		t.log.Infof("feeding %s", t.feedClient)
		t.feedPinger = t.feedClient.NewPinger()
		time.Sleep(time.Microsecond * 10)
		t.feedPinger.Start(t.ctx, FeedPingerInterval)
	}
}

func (t *T) onInstanceStatusDeleted(c *msgbus.InstanceStatusDeleted) {
	i := instance.InstanceString(c.Path, c.Node)
	delete(t.changes.instanceStatusUpdates, i)
	delete(t.instances, i)
	t.changes.instanceStatusDeletes[i] = c
	// TODO: confirm we must update daemonStatusChange on event ?
	t.daemonStatusChange[i] = struct{}{}
}

func (t *T) onInstanceStatusUpdated(c *msgbus.InstanceStatusUpdated) {
	i := instance.InstanceString(c.Path, c.Node)
	delete(t.changes.instanceStatusDeletes, i)
	t.changes.instanceStatusUpdates[i] = c
	t.instances[i] = struct{}{}
	t.daemonStatusChange[i] = struct{}{}
}

func (t *T) onNodeConfigUpdated(c *msgbus.NodeConfigUpdated) {
	t.onConfigUpdated()
}

func (t *T) onNodeMonitorDeleted(c *msgbus.NodeMonitorDeleted) {
	// TODO: confirm we must update daemonStatusChange on event ?
	t.daemonStatusChange[c.Node] = struct{}{}
}

func (t *T) onNodeStatusUpdated(c *msgbus.NodeStatusUpdated) {
	t.daemonStatusChange[c.Node] = struct{}{}
}

func (t *T) onObjectStatusDeleted(c *msgbus.ObjectStatusDeleted) {
	// TODO: confirm we must update daemonStatusChange on event ?
	t.daemonStatusChange[c.Path.String()] = struct{}{}
}

func (t *T) onObjectStatusUpdated(c *msgbus.ObjectStatusUpdated) {
	t.daemonStatusChange[c.Path.String()] = struct{}{}
}

func (t *T) sendBeginAction(data []string) {
	t.feedClient.Call("begin_action", Headers, data)
}

func (t *T) sendLogs(data [][]string) {
	t.feedClient.Call("res_action_batch", Headers, data)
}

func (t *T) sendCollectorData() error {
	if t.featurePostChange {
		return t.sendCollectorDataFeatureChange()
	} else {
		return t.sendCollectorDataLegacy()
	}
}

func (t *T) sendCollectorDataFeatureChange() error {
	if t.hasChanges() {
		return t.postChanges()
	} else {
		return t.postPing()
	}
}

func (t *T) sendCollectorDataLegacy() error {
	if t.hasDaemonStatusChange() {
		return t.postStatus()
	} else {
		return t.postPing()
	}
}

func (t *T) hasChanges() bool {
	change := t.changes
	if len(change.instanceStatusUpdates) > 0 {
		return true
	}
	if len(change.instanceStatusDeletes) > 0 {
		return true
	}
	return false
}

func (t *T) hasDaemonStatusChange() bool {
	return len(t.daemonStatusChange) > 0
}

func (t *T) postPing() error {
	instances := make([]string, 0, len(t.instances))
	for k := range t.instances {
		instances = append(instances, k)
	}
	now := time.Now()
	method := http.MethodPost
	path := "/oc3/feed/daemon/ping"
	t.log.Debugf("%s %s", method, path)
	resp, err := t.client.DoRequest(method, path, nil)
	if err != nil {
		return err
	}
	t.log.Debugf("%s %s status code %d", method, path, resp.StatusCode)
	switch resp.StatusCode {
	case http.StatusNoContent:
		// collector detect out of sync
		t.initChanges()
		t.previousUpdatedAt = time.Time{}
		return nil
	case http.StatusAccepted:
		// collector accept changes, we can drop pending change
		t.previousUpdatedAt = now
		t.dropChanges()
		return nil
	default:
		return fmt.Errorf("%s %s unexpected status code %d", method, path, resp.StatusCode)
	}
}

func (t *T) postChanges() error {
	var (
		ioReader io.Reader
		method   = http.MethodPost
		path     = "/oc3/feed/daemon/changes"
	)
	now := time.Now()

	if b, err := t.changes.asPostBody(t.previousUpdatedAt, now); err != nil {
		return fmt.Errorf("post daemon change body: %s", err)
	} else {
		ioReader = bytes.NewBuffer(b)
	}

	t.log.Debugf("%s %s from %s -> %s", method, path, t.previousUpdatedAt, now)
	resp, err := t.client.DoRequest(method, path, ioReader)
	if err != nil {
		return fmt.Errorf("post daemon change call: %w", err)
	}

	t.log.Debugf("post daemon change status code %d", resp.StatusCode)
	switch resp.StatusCode {
	case http.StatusConflict:
		// collector detect out of sync (collector previousUpdatedAt is not t.previousUpdatedAt), recreate full
		t.initChanges()
		t.previousUpdatedAt = time.Time{}
		return nil
	case http.StatusAccepted:
		// collector accept changes, we can drop pending change
		t.previousUpdatedAt = now
		t.dropChanges()
		return nil
	default:
		t.log.Warnf("post daemon change unexpected status code %d", resp.StatusCode)
		return fmt.Errorf("post daemon change unexpected status code %d", resp.StatusCode)
	}
}

func (t *T) postStatus() error {
	var (
		req      *http.Request
		resp     *http.Response
		err      error
		ioReader io.Reader
		method   = http.MethodPost
		path     = "/oc3/feed/daemon/status"
	)
	now := time.Now()
	body := statusPost{
		previousUpdatedAt: t.previousUpdatedAt,
		UpdatedAt:         now,
		Data:              t.clusterData.ClusterData(),
	}
	if body.Data == nil {
		return fmt.Errorf("%s %s abort on empty cluster data", method, path)
	}
	if b, err := json.Marshal(body); err != nil {
		return fmt.Errorf("post daemon status body: %s", err)
	} else {
		ioReader = bytes.NewBuffer(b)
		t.log.Infof("%s %s from %s -> %s change len: %d [%s]", method, path, t.previousUpdatedAt, now, len(t.daemonStatusChange), strings.Join(xmap.Keys(t.daemonStatusChange), " "))
	}

	t.log.Debugf("%s %s from %s -> %s", method, path, t.previousUpdatedAt, now)
	req, err = t.client.NewRequest(method, path, ioReader)
	if err != nil {
		return fmt.Errorf("%s %s create request: %w", method, path, err)
	}

	req.Header.Set(headerPreviousUpdatedAt, t.previousUpdatedAt.Format(time.RFC3339Nano))
	req.Header.Set(headerDaemonChange, strings.Join(xmap.Keys(t.daemonStatusChange), " "))

	resp, err = t.client.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}

	t.log.Infof("%s %s status code %d", method, path, resp.StatusCode)
	switch resp.StatusCode {
	case http.StatusConflict:
		// collector detect out of sync (collector previousUpdatedAt is not t.previousUpdatedAt), recreate full
		t.initChanges()
		t.previousUpdatedAt = time.Time{}
		return nil
	case http.StatusAccepted:
		// collector accept changes, we can drop pending change
		t.previousUpdatedAt = now
		t.dropChanges()
		return nil
	default:
		t.log.Warnf("%s %s unexpected status code %d", method, path, resp.StatusCode)
		return fmt.Errorf("%s %s unexpected status code %d", method, path, resp.StatusCode)
	}
}

func (c *changesData) asPostBody(previous, current time.Time) ([]byte, error) {
	iStatusChanges := make([]msgbus.InstanceStatusUpdated, 0, len(c.instanceStatusUpdates))
	for _, v := range c.instanceStatusUpdates {
		iStatusChanges = append(iStatusChanges, *v)
	}
	iStatusDeletes := make([]msgbus.InstanceStatusDeleted, 0, len(c.instanceStatusDeletes))
	for _, v := range c.instanceStatusDeletes {
		iStatusDeletes = append(iStatusDeletes, *v)
	}

	return json.Marshal(changesPost{
		PreviousUpdatedAt:     previous,
		UpdatedAt:             current,
		InstanceStatusUpdates: iStatusChanges,
		InstanceStatusDeletes: iStatusDeletes,
	})
}

func (t *T) initChanges() {
	t.previousUpdatedAt = time.Time{}
	t.instances = make(map[string]struct{})
	t.changes = changesData{
		instanceStatusUpdates: make(map[string]*msgbus.InstanceStatusUpdated),
		instanceStatusDeletes: make(map[string]*msgbus.InstanceStatusDeleted),
	}
	t.daemonStatusChange = make(map[string]struct{})

	for _, v := range instance.StatusData.GetAll() {
		i := instance.InstanceString(v.Path, v.Node)
		t.instances[i] = struct{}{}
		t.changes.instanceStatusUpdates[i] = &msgbus.InstanceStatusUpdated{
			Path:  v.Path,
			Node:  v.Node,
			Value: *v.Value,
		}

		t.daemonStatusChange[v.Path.String()] = struct{}{}
		// TODO: use object cache ?
		t.daemonStatusChange[i] = struct{}{}
		// TODO: use node cache ?
		t.daemonStatusChange[v.Node] = struct{}{}
	}
}

func (t *T) dropChanges() {
	t.changes.instanceStatusUpdates = make(map[string]*msgbus.InstanceStatusUpdated)
	t.changes.instanceStatusDeletes = make(map[string]*msgbus.InstanceStatusDeleted)
	t.daemonStatusChange = make(map[string]struct{})
}
