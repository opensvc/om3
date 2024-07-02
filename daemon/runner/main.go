package runner

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/opensvc/om3/core/priority"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/prioqueue"
	"github.com/opensvc/om3/util/pubsub"
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
		bus        *pubsub.Bus
		subQS      pubsub.QueueSizer
		log        *plog.Logger
	}
)

func (t Item) Before(other prioqueue.Interface) bool {
	return t.priority < other.(Item).priority
}

var (
	def *T
)

func (t *T) startSubscriptions() *pubsub.Subscription {
	t.bus = pubsub.BusFromContext(t.ctx)
	sub := t.bus.Sub("daemon.runner", t.subQS)
	labelLocalhost := pubsub.Label{"node", hostname.Hostname()}
	sub.AddFilter(&msgbus.NodeConfigUpdated{}, labelLocalhost)
	sub.Start()
	return sub
}

// TODO: expose queue len via prometheus
func (t *T) run() {
	for {
		running := t.running.Load()
		if running >= int32(t.maxRunning) {
			//t.log.Debugf("priority run queue full: %d running %d waiting", running, t.queue.Len())
			return
		}
		i := t.queue.Pop()
		if i == nil {
			//t.log.Debugf("priority run queue empty")
			return
		}
		item := i.(Item)
		t.running.Add(1)
		//t.log.Debugf("priority run dequeue from p%d: %d running %d waiting", item.priority, running, t.queue.Len())
		go func() {
			item.errC <- item.f()
			t.running.Add(-1)
		}()
	}
}

func (t *T) do(ctx context.Context) {
	defer t.wg.Done()
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()
	sub := t.startSubscriptions()
	defer sub.Stop()
	for {
		select {
		case ev := <-sub.C:
			switch c := ev.(type) {
			case *msgbus.NodeConfigUpdated:
				t.maxRunning = c.Value.MaxParallel
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

func NewDefault(subQS pubsub.QueueSizer) *T {
	if def == nil {
		def = New(subQS)
	}
	return def
}

func New(subQS pubsub.QueueSizer) *T {
	return &T{
		interval:   200 * time.Millisecond,
		maxRunning: 5,

		stage: make(chan Item),
		queue: prioqueue.New(),
		subQS: subQS,
		log:   plog.NewDefaultLogger().Attr("pkg", "runner"),
	}
}

func (t *T) Stop() error {
	t.cancel()
	t.wg.Wait()
	return nil
}

func (t *T) Start(ctx context.Context) error {
	t.ctx, t.cancel = context.WithCancel(ctx)
	t.wg.Add(1)
	go t.do(ctx)
	return nil
}

func (t *T) Run(p priority.T, f func() error) error {
	item := Item{
		f:        f,
		priority: p,
		errC:     make(chan error),
	}
	t.stage <- item
	return <-item.errC
}

func (t *T) SetMaxRunning(n int) {
	t.maxRunning = n
}

func Start(ctx context.Context) {
	def.Start(ctx)
}

func Run(p priority.T, f func() error) error {
	return def.Run(p, f)
}

func SetMaxRunning(n int) {
	def.SetMaxRunning(n)
}
