package collector

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/opensvc/om3/core/collector"
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
		wg         sync.WaitGroup
		bus        *pubsub.Bus
		created    map[string]time.Time
		sendTicker *time.Ticker
		lastSend   time.Time
		toSend     [][]string
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
	sub.AddFilter(&msgbus.NodeConfigUpdated{}, labelLocalhost)
	sub.Start()
	return sub
}

func (t *T) loop() {
	t.log.Infof("loop started")
	sub := t.startSubscriptions()
	defer func() {
		if err := sub.Stop(); err != nil {
			t.log.Errorf("subscription stop: %s", err)
		}
	}()

	for {
		select {
		case ev := <-sub.C:
			switch c := ev.(type) {
			case *msgbus.ClusterConfigUpdated:
				t.onClusterConfigUpdated(c)
			case *msgbus.NodeConfigUpdated:
				t.onNodeConfigUpdated(c)
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
	if err != nil {
		t.log.Infof("the collector routine is dormant: %s", err)
	} else {
		t.log.Infof("feeding %s", t.feedClient)
		t.feedPinger = t.feedClient.NewPinger()
		time.Sleep(time.Microsecond * 10)
		t.feedPinger.Start(t.ctx, FeedPingerInterval)
	}
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
