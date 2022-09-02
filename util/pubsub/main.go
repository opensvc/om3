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
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
		fn       func(interface{})
		op       uint
		ns       uint
		matching string
		name     string
		q        chan interface{}
	}
)

type (
	cmdDie struct {
	}

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
)

var (
	bus *Bus
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
		subs := make(map[uuid.UUID]activeSubscription)
		started <- true
		for {
			select {
			case <-b.ctx.Done():
				return
			case cmd := <-b.cmdC:
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
					sub := activeSubscription{
						name:     c.name,
						ns:       c.ns,
						op:       c.op,
						matching: c.matching,
						fn:       c.fn,
						q:        make(chan interface{}, 100),
					}
					subs[id] = sub
					started := make(chan bool)
					b.Add(1)
					go func() {
						b.Done()
						started <- true
						for {
							select {
							case i := <-sub.q:
								if _, ok := i.(cmdDie); ok {
									return
								}
								sub.fn(i)
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
						continue
					}
					sub.q <- cmdDie{}
					delete(subs, c.subId)
					select {
					case <-b.ctx.Done():
					case c.resp <- sub.name:
					}
					b.log.Debug().Msgf("unsubscribe %s", sub.name)
				}
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
