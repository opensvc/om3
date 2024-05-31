package collector

import (
	"context"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/opensvc/om3/core/collector"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/rawconfig"
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

		postTicker *time.Ticker

		// sentAt is the timestamp of the last successfully data sent to
		// collector. It is used by collector to detect out of sync data.
		sentAt time.Time

		// changes is used to POST /daemon/data
		changes changesData

		// instances is used to POST /daemon/ping
		instances map[string]struct{}
	}

	requester interface {
		DoRequest(method string, relPath string, body io.Reader) (*http.Response, error)
	}

	changesData struct {
		instanceStatusUpdates map[string]*msgbus.InstanceStatusUpdated
		instanceStatusDeletes map[string]*msgbus.InstanceStatusDeleted
	}

	postData struct {
		instanceStatusUpdates []msgbus.InstanceStatusUpdated
		InstanceStatusDeletes []msgbus.InstanceStatusDeleted
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

func New(opts ...funcopt.O) *T {
	t := &T{
		log:       plog.NewDefaultLogger().WithPrefix("daemon: collector: ").Attr("pkg", "daemon/collector"),
		localhost: hostname.Hostname(),
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
			}
		case <-refreshTicker.C:
			t.sendCollectorData()
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
		t.log.Errorf("can't setup requester: %w", err)
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
}

func (t *T) onInstanceStatusUpdated(c *msgbus.InstanceStatusUpdated) {
	i := instance.InstanceString(c.Path, c.Node)
	delete(t.changes.instanceStatusDeletes, i)
	t.changes.instanceStatusUpdates[i] = c
	t.instances[i] = struct{}{}
}

func (t *T) onNodeConfigUpdated(c *msgbus.NodeConfigUpdated) {
	t.onConfigUpdated()
}

func (t *T) sendBeginAction(data []string) {
	t.feedClient.Call("begin_action", Headers, data)
}

func (t *T) sendLogs(data [][]string) {
	t.feedClient.Call("res_action_batch", Headers, data)
}

func (t *T) sendCollectorData() {
	if t.hasChanges() {
		t.postChanges()
		return
	} else {
		t.postPing()
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

func (t *T) postPing() {
	instances := make([]string, 0, len(t.instances))
	for k := range t.instances {
		instances = append(instances, k)
	}
	now := time.Now()
	t.log.Infof("POST /daemon/ping from %s -> %s: %#v", t.sentAt, now, instances)
}

func (t *T) postChanges() {
	now := time.Now()
	dPost := t.changes.asLists()
	t.log.Infof("POST /daemon/change changes from %s -> %s: %#v", t.sentAt, now, dPost)
	postCode := http.StatusAccepted
	switch postCode {
	case http.StatusConflict:
		// collector detect out of sync (collector sentAt is not t.sentAt), recreate full
		t.initChanges()
		t.sentAt = time.Time{}
	case http.StatusAccepted:
		// collector accept changes, we can drop pending change
		t.sentAt = now
		t.dropChanges()
	}
}

func (c *changesData) asLists() postData {
	iStatusChanges := make([]msgbus.InstanceStatusUpdated, 0, len(c.instanceStatusUpdates))
	for _, v := range c.instanceStatusUpdates {
		iStatusChanges = append(iStatusChanges, *v)
	}
	iStatusDeletes := make([]msgbus.InstanceStatusDeleted, 0, len(c.instanceStatusDeletes))
	for _, v := range c.instanceStatusDeletes {
		iStatusDeletes = append(iStatusDeletes, *v)
	}

	dPost := postData{
		instanceStatusUpdates: iStatusChanges,
		InstanceStatusDeletes: iStatusDeletes,
	}
	return dPost
}

func (t *T) initChanges() {
	t.sentAt = time.Time{}
	t.instances = make(map[string]struct{})
	t.changes = changesData{
		instanceStatusUpdates: make(map[string]*msgbus.InstanceStatusUpdated),
		instanceStatusDeletes: make(map[string]*msgbus.InstanceStatusDeleted),
	}

	for _, v := range instance.StatusData.GetAll() {
		i := instance.InstanceString(v.Path, v.Node)
		t.instances[i] = struct{}{}
		t.changes.instanceStatusUpdates[i] = &msgbus.InstanceStatusUpdated{
			Path:  v.Path,
			Node:  v.Node,
			Value: *v.Value,
		}
	}
}

func (t *T) dropChanges() {
	t.changes.instanceStatusUpdates = make(map[string]*msgbus.InstanceStatusUpdated)
	t.changes.instanceStatusDeletes = make(map[string]*msgbus.InstanceStatusDeleted)
}
