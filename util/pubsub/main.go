// Package pubSub implements simple pub-sub
//
// Example:
//	  import (
//    	"context"
//    	"fmt"
//
//    	"opensvc.com/opensvc/util/pubSub"
//    )
//
//    func main() {
//    	const (
//    		NsNum1 = pubSub.NsAll + 1 + iota
//    		NsNum2
//    	)
//
//      ctx, cancel := context.WithCancel(context.Background())
//      defer cancel()
//
//  	// Start the pub-sub
//      c := pubSub.Start(ctx, "pub-sub example")
//
//    	// Prepare a new subscription details
//    	subOnCreate := pubSub.Subscription{
//    		Ns:       NsNum1,
//    		Op:       pubSub.OpCreate,
//    		Matching: "idA",
//    		Name:     "subscription example",
//    	}
//
//    	// register the subscription
//    	sub1Id := pubSub.Sub(c, subOnCreate, func(i interface{}) {
//    		fmt.Printf("detected from subscription 1: value '%s' has been published with operation 'OpCreate' on id 'IdA' in name space 'NsNum1'\n", i)
//    	})
//    	defer pubSub.Unsub(c, sub1Id)
//
//    	// register another subscription that watch all namespaces/operations/ids
//    	defer pubSub.Unsub(
//    		c,
//    		pubSub.Sub(c,
//    			pubSub.Subscription{Name: "watch all"},
//    			func(i interface{}) {
//    				fmt.Printf("detected from subscription 2: value '%s' have been published\n", i)
//    			}))
//
//    	// publish a create operation of "something" on namespace NsNum1
//    	pubSub.Pub(c, pubSub.Publication{
//    		Ns:    NsNum1,
//    		Op:    pubSub.OpCreate,
//    		Id:    "idA",
//    		Value: "foo bar",
//    	})
//
//    	// publish a Update operation of "a value" on namespace NsNum2
//    	pubSub.Pub(c, pubSub.Publication{
//    		Ns:    NsNum2,
//    		Op:    pubSub.OpUpdate,
//    		Id:    "idXXXX",
//    		Value: "a value",
//    	})
//    }
//

package pubsub

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/util/durationlog"
)

type (
	contextKey int
)

const (
	// OpAll can be used on Subscription to subscribe on all operations
	OpAll = iota
	OpCreate
	OpRead
	OpUpdate
	OpDelete
)

const (
	// NsAll operation value can be used for all name spaces
	NsAll = iota

	busContextKey contextKey = 0
)

type (
	// Subscription struct holds a subscription details
	Subscription struct {
		// Ns is the namespace to subscribe on
		Ns uint

		// Op is operation to subscribe on
		Op uint

		// Matching is the publication id to subscribe on
		// zero value means subscription on all Publications Id
		Matching string

		// Name is a description of the subscription
		Name string

		// Timeout define the subscription max duration for the callback
		// when non 0, if subscription callback duration exceed Timeout, the
		// subscription is removed
		Timeout time.Duration
	}

	// Publication struct holds a new publication
	Publication struct {
		// Ns it the publication namespace
		Ns uint
		// Op is the publication operation
		Op uint

		// Id is the publication Id (used by Subscription)
		Id string

		// Value is the thing to publish
		Value interface{}
	}

	activeSubscription struct {
		fn       func(interface{}) error
		op       uint
		ns       uint
		matching string
		name     string
		q        chan interface{}

		// duration define the max duration for a callback,
		// when non 0, the subscription is kill if callback duration exceeds duration
		duration time.Duration
		subId    uuid.UUID

		// cancel defines the subscription canceler
		cancel context.CancelFunc
	}
)

type (
	cmdPub struct {
		id   string
		op   uint
		ns   uint
		data interface{}
		resp chan<- bool
	}

	cmdSub struct {
		fn       func(interface{})
		op       uint
		ns       uint
		matching string
		name     string
		resp     chan<- uuid.UUID
		timeout  time.Duration
	}

	cmdUnsub struct {
		subId uuid.UUID
		resp  chan<- string
	}

	Bus struct {
		sync.WaitGroup
		name   string
		cmdC   chan interface{}
		cancel func()
		log    zerolog.Logger
		ctx    context.Context
	}

	stringer interface {
		String() string
	}
)

var (
	bus *Bus

	cmdDurationWarn    = time.Second
	notifyDurationWarn = time.Second

	OpToName = []string{"all operations", "create", "read", "update", "delete"}
)

// Stop stops the default bus
func Stop() {
	bus.Stop()
}

// Start starts the default bus
func Start(ctx context.Context) {
	if bus == nil {
		bus = NewBus("default")
	}
	bus.Start(ctx)
}

// StartBus allocate and runs a new Bus and return a pointer
func NewBus(name string) *Bus {
	b := &Bus{}
	b.name = name
	b.cmdC = make(chan interface{})
	b.log = log.Logger.With().Str("bus", name).Logger()
	return b
}

func (b *Bus) Start(ctx context.Context) {
	b.ctx, b.cancel = context.WithCancel(ctx)
	started := make(chan bool)
	b.Empty() // flush cmds queued while we were stopped ?
	b.Add(1)
	go func() {
		defer b.Done()

		watchDuration := &durationlog.T{Log: b.log}
		watchDurationCtx, watchDurationCancel := context.WithCancel(context.Background())
		defer watchDurationCancel()
		var beginCmd = make(chan interface{})
		var endCmd = make(chan bool)
		b.Add(1)
		go func() {
			defer b.Done()
			watchDuration.WarnExceeded(watchDurationCtx, beginCmd, endCmd, cmdDurationWarn, "msg")
		}()

		subs := make(map[uuid.UUID]activeSubscription)
		started <- true
		for {
			select {
			case <-b.ctx.Done():
				return
			case cmd := <-b.cmdC:
				beginCmd <- cmd
				switch c := cmd.(type) {
				case cmdPub:
					for _, sub := range subs {
						if sub.ns != NsAll && sub.ns != c.ns {
							continue
						}
						if sub.op != OpAll && sub.op != c.op {
							continue
						}
						if sub.matching != "" && sub.matching != c.id {
							continue
						}
						b.log.Debug().Msgf("route %#v to subscriber %s", c.data, sub.name)
						sub.q <- c.data
					}
					select {
					case <-b.ctx.Done():
					case c.resp <- true:
					}
				case cmdSub:
					id := uuid.New()
					subCtx, subCtxCancel := context.WithCancel(context.Background())
					sub := activeSubscription{
						name:     c.name,
						ns:       c.ns,
						op:       c.op,
						matching: c.matching,
						fn:       createCallBack(c.fn, c.timeout),
						q:        make(chan interface{}, 100),
						subId:    id,
						duration: c.timeout,
						cancel:   subCtxCancel,
					}
					subs[id] = sub
					started := make(chan bool)
					b.Add(1)
					go func() {
						b.Done()
						defer sub.cancel()
						defer func() {
							// empty any pending message for this subscription
							ticker := time.NewTicker(2 * time.Second)
							defer ticker.Stop()
							for {
								select {
								case <-sub.q:
								case <-ticker.C:
									return
								}
							}
						}()
						watchSubscription := &durationlog.T{Log: b.log}
						var beginNotify = make(chan interface{})
						var endNotify = make(chan bool)
						b.Add(1)
						go func() {
							defer b.Done()
							watchSubscription.WarnExceeded(subCtx, beginNotify, endNotify, notifyDurationWarn, "msg: notify: '"+c.name+"'")
						}()
						started <- true
						for {
							select {
							case <-subCtx.Done():
								return
							case i := <-sub.q:
								beginNotify <- i
								startTime := time.Now()
								if sub.fn(i) != nil {
									// the subscription is too slow, kill it
									// then ask for unsubcribe
									b.log.Warn().Msgf("max duration exceeded %.02fs: msg: notify kill: %s: '%s'",
										time.Now().Sub(startTime).Seconds(), sub.subId, sub.name)
									sub.cancel()
									go b.Unsub(sub.subId)
									return
								}
								endNotify <- true
							case <-b.ctx.Done():
								return
							}
						}
					}()
					<-started
					c.resp <- id
					b.log.Debug().Msgf("subscribe %s", sub.name)
				case cmdUnsub:
					sub, ok := subs[c.subId]
					if !ok {
						break
					}
					sub.cancel()
					delete(subs, c.subId)
					select {
					case <-b.ctx.Done():
					case c.resp <- sub.name:
					}
					b.log.Debug().Msgf("unsubscribe %s", sub.name)
				}
				endCmd <- true
			}
		}
	}()
	<-started
	b.log.Info().Msg("started")
}

func (b *Bus) Empty() {
	defer b.log.Info().Msg("empty channel")
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-b.cmdC:
		case <-ticker.C:
			return
		}
	}
}

func (b *Bus) Stop() {
	if b == nil {
		return
	}
	if b.cancel != nil {
		f := b.cancel
		b.cancel = nil
		f()
		b.Wait()
		b.log.Info().Msg("stopped")
		b.Empty()
	}
}

// Pub function publish a new p Publication
func (b Bus) Pub(p Publication) {
	done := make(chan bool)
	op := cmdPub{
		id:   p.Id,
		op:   p.Op,
		ns:   p.Ns,
		data: p.Value,
		resp: done,
	}
	select {
	case b.cmdC <- op:
	case <-b.ctx.Done():
		return
	}
	select {
	case <-done:
		return
	case <-b.ctx.Done():
		return
	}
}

// Sub function submit a new Subscription on pub-sub
// It returns the subscription uuid.UUID (can be used to un-subscribe)
func (b Bus) Sub(s Subscription, fn func(interface{})) uuid.UUID {
	respC := make(chan uuid.UUID)
	op := cmdSub{
		fn:       fn,
		op:       s.Op,
		ns:       s.Ns,
		matching: s.Matching,
		name:     s.Name,
		resp:     respC,
		timeout:  s.Timeout,
	}
	select {
	case b.cmdC <- op:
	case <-b.ctx.Done():
		return uuid.UUID{}
	}
	select {
	case uuid := <-respC:
		return uuid
	case <-b.ctx.Done():
		return uuid.UUID{}
	}
}

// Unsub function remove a subscription
func (b Bus) Unsub(id uuid.UUID) string {
	respC := make(chan string)
	op := cmdUnsub{
		subId: id,
		resp:  respC,
	}
	select {
	case b.cmdC <- op:
	case <-b.ctx.Done():
		return ""
	}
	select {
	case s := <-respC:
		return s
	case <-b.ctx.Done():
		return ""
	}
}

// ContextWithBus stores the bus in the context and returns the new context.
func ContextWithBus(ctx context.Context, bus *Bus) context.Context {
	return context.WithValue(ctx, busContextKey, bus)
}

func BusFromContext(ctx context.Context) *Bus {
	if bus, ok := ctx.Value(busContextKey).(*Bus); ok {
		return bus
	}
	panic("unable to retrieve pubsub bus from context")
}

// createCallBack returns wrapper to f that will return error when f duration exceeds timeout
// When timeout is 0, wrapper will always return nil
func createCallBack(f func(interface{}), timeout time.Duration) func(interface{}) error {
	if timeout == 0 {
		return func(i interface{}) error {
			f(i)
			return nil
		}
	} else {
		return func(i interface{}) error {
			done := make(chan bool)
			timer := time.NewTimer(timeout)
			go func() {
				f(i)
				done <- true
			}()
			select {
			case <-done:
				timer.Stop()
				return nil
			case <-timer.C:
				return errors.New("timeout")
			}
		}
	}
}

func (o cmdPub) String() string {
	var dataS string
	switch data := o.data.(type) {
	case stringer:
		dataS = data.String()
	default:
		dataS = reflect.TypeOf(data).String()
	}
	return fmt.Sprintf("publish: id '%s' %s on namespace %d data: %s", o.id, OpToName[o.op], o.ns, dataS)
}

func (o cmdSub) String() string {
	return fmt.Sprintf("subscribe: '%s' for %d on namespace %d", o.name, OpToName[o.op], o.ns)
}

func (o cmdUnsub) String() string {
	return fmt.Sprintf("unsubscribe: id '%s'", o.subId)
}
