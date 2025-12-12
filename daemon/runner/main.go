package runner

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/priority"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/prioqueue"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type (
	Item struct {
		f        func() error
		priority priority.T
		errC     chan error
	}

	T struct {
		running    atomic.Int32
		maxRunning int
		interval   time.Duration
		stage      chan Item
		queue      *prioqueue.Queue
		wg         sync.WaitGroup
		ctx        context.Context
		cancel     context.CancelFunc
		publisher  pubsub.Publisher
		subQS      pubsub.QueueSizer
		log        *plog.Logger
		status     daemonsubsystem.RunnerImon
	}
)

func (t Item) Before(other prioqueue.Interface) bool {
	return t.priority < other.(Item).priority
}

var (
	def *T
)

func (t *T) startSubscriptions() *pubsub.Subscription {
	sub := pubsub.SubFromContext(t.ctx, "daemon.runner", t.subQS)
	labelLocalhost := pubsub.Label{"node", hostname.Hostname()}
	sub.AddFilter(&msgbus.NodeConfigUpdated{}, labelLocalhost)
	sub.Start()
	return sub
}

// TODO: expose queue len via prometheus
func (t *T) run() {
	imStarted := make(chan bool)
	for {
		running := t.running.Load()
		if running >= int32(t.maxRunning) {
			//t.log.Tracef("priority run queue full: %d running %d waiting", running, t.queue.Len())
			return
		}
		i := t.queue.Pop()
		if i == nil {
			//t.log.Tracef("priority run queue empty")
			return
		}
		item := i.(Item)
		t.running.Add(1)
		//t.log.Tracef("priority run dequeue from p%d: %d running %d waiting", item.priority, running, t.queue.Len())
		go func() {
			imStarted <- true
			err := item.f()
			if item.errC != nil {
				item.errC <- err
			}
			t.running.Add(-1)
		}()
		<-imStarted
	}
}

func (t *T) do(ctx context.Context) {
	ticker := time.NewTicker(t.interval)

	t.publisher = pubsub.PubFromContext(ctx)
	sub := t.startSubscriptions()
	t.log.Infof("started")
	defer func() {
		sub.Stop()
		ticker.Stop()
		t.ctx = nil
		t.cancel = nil
		t.wg.Done()
		t.log.Infof("stopped")
	}()

	if nodeConfig := node.ConfigData.GetByNode(hostname.Hostname()); nodeConfig != nil {
		if nodeConfig.MaxParallel > 0 {
			t.maxRunning = nodeConfig.MaxParallel
			t.status.MaxRunning = t.maxRunning
		} else {
			t.log.Warnf("ignore node config with MaxParallel value 0")
		}
	}

	t.publishUpdate()
	t.log.Infof("started with interval %s, max running: %d", t.interval, t.maxRunning)

	for {
		select {
		case ev := <-sub.C:
			switch c := ev.(type) {
			case *msgbus.NodeConfigUpdated:
				if c.Value.MaxParallel > 0 {
					t.maxRunning = c.Value.MaxParallel

					if t.status.MaxRunning != t.maxRunning {
						t.log.Infof("max running changed %d -> %d", t.status.MaxRunning, t.maxRunning)
						t.status.MaxRunning = t.maxRunning
						t.publishUpdate()
					}
				} else {
					t.log.Warnf("on NodeConfigUpdated ignore MaxParallel value 0")
				}
			}
		case item := <-t.stage:
			// serialize pushes
			t.queue.Push(item)
		case <-ticker.C:
			t.run()
		case <-ctx.Done():
			// clean up ?
			return
		}
	}
}

func SetDefault(t *T) {
	if def != nil {
		panic("a default runner is already installed")
	}
	def = t
}

func NewDefault(subQS pubsub.QueueSizer) *T {
	if def != nil {
		return def
	}
	def = New(subQS)
	return def
}

func New(subQS pubsub.QueueSizer) *T {
	return &T{
		interval:   200 * time.Millisecond,
		maxRunning: 5,

		stage: make(chan Item),
		queue: prioqueue.New(),
		subQS: subQS,
		log:   plog.NewDefaultLogger().Attr("pkg", "runner_imon").WithPrefix("daemon: runner_imon: "),

		status: daemonsubsystem.RunnerImon{
			Status: daemonsubsystem.Status{CreatedAt: time.Now(), ID: "runner_imon"},
		},
	}
}

func (t *T) Stop() error {
	t.cancel()
	t.wg.Wait()
	return nil
}

func (t *T) Start(ctx context.Context) error {
	if t.ctx != nil {
		return nil
	}
	t.ctx, t.cancel = context.WithCancel(ctx)
	t.wg.Add(1)
	t.status.State = "running"
	go t.do(t.ctx)
	return nil
}

func (t *T) Enqueue(p priority.T, errC chan error, f func() error) {
	item := Item{
		f:        f,
		priority: p,
		errC:     errC,
	}
	t.stage <- item
}

func (t *T) Run(p priority.T, f func() error) error {
	errC := make(chan error)
	t.Enqueue(p, errC, f)
	return <-errC
}

func (t *T) SetMaxRunning(n int) {
	t.maxRunning = n
}

func (t *T) SetInterval(d time.Duration) {
	t.interval = d
}

func (t *T) publishUpdate() {
	t.status.UpdatedAt = time.Now()
	localhost := hostname.Hostname()
	daemonsubsystem.DataRunnerImon.Set(localhost, t.status.DeepCopy())
	t.publisher.Pub(&msgbus.DaemonRunnerImonUpdated{Node: localhost, Value: *t.status.DeepCopy()}, pubsub.Label{"node", localhost})
}

func Stop() error {
	return def.Stop()
}

func Start(ctx context.Context) error {
	return def.Start(ctx)
}

func Run(p priority.T, f func() error) error {
	return def.Run(p, f)
}

func Enqueue(p priority.T, errC chan error, f func() error) {
	def.Enqueue(p, errC, f)
}

func SetMaxRunning(n int) {
	def.SetMaxRunning(n)
}
