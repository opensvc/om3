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
	"time"

	"github.com/google/uuid"

	"opensvc.com/opensvc/daemon/daemonlogctx"
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
	}

	cmdUnsub struct {
		subId uuid.UUID
		resp  chan<- string
	}
)

// Start runs a new pub sub
//
// returns the created pub sub cmd chan
//
func Start(ctx context.Context, name string) chan<- interface{} {
	log := daemonlogctx.Logger(ctx).With().Str("_pkg", "pubSub").Str("name", name).Logger()
	started := make(chan struct{})
	cmdC := make(chan interface{})
	go func() {
		subs := make(map[uuid.UUID]func(interface{}))
		subNames := make(map[uuid.UUID]string)
		subNs := make(map[uuid.UUID]uint)
		subOps := make(map[uuid.UUID]uint)
		subMatching := make(map[uuid.UUID]string)
		subQueue := make(map[uuid.UUID]chan interface{})
		defer func() {
			go func() {
				log.Info().Msg("stopping")
				defer log.Info().Msg("stopped")
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()
				for {
					select {
					case <-cmdC:
					case <-ctx.Done():
						return
					}
				}
			}()
		}()
		started <- struct{}{}
		for {
			select {
			case <-ctx.Done():
				return
			case cmd := <-cmdC:
				switch c := cmd.(type) {
				case cmdPub:
					for id := range subs {
						if subNs[id] != NsAll && subNs[id] != c.ns {
							continue
						}
						if subOps[id] != OpAll && subOps[id] != c.op {
							continue
						}
						if len(subMatching[id]) != 0 && subMatching[id] != c.id {
							continue
						}
						subQueue[id] <- c.data
					}
					c.resp <- true
				case cmdSub:
					id := uuid.New()
					subs[id] = c.fn
					subNames[id] = c.name
					subNs[id] = c.ns
					subOps[id] = c.op
					subMatching[id] = c.matching
					queue := make(chan interface{}, 100)
					subQueue[id] = queue
					fn := c.fn
					started := make(chan struct{})
					go func() {
						started <- struct{}{}
						for i := range queue {
							fn(i)
						}
					}()
					<-started
					c.resp <- id
					log.Debug().Msgf("subscribe %s", c.name)
				case cmdUnsub:
					name, ok := subNames[c.subId]
					if !ok {
						continue
					}
					queue := subQueue[c.subId]
					delete(subs, c.subId)
					delete(subNames, c.subId)
					delete(subNs, c.subId)
					delete(subOps, c.subId)
					delete(subQueue, c.subId)
					// end subscriber dispatcher
					close(queue)
					c.resp <- name
					log.Debug().Msgf("unsubscribe %s", name)
				}
			}
		}
	}()
	<-started
	log.Info().Msg("started")
	return cmdC
}

// Pub function publish a new p Publication
func Pub(cmdC chan<- interface{}, p Publication) {
	done := make(chan bool)
	cmdC <- cmdPub{
		id:   p.Id,
		op:   p.Op,
		ns:   p.Ns,
		data: p.Value,
		resp: done,
	}
	<-done
}

// Sub function submit a new Subscription on pub-sub
// It returns the subscription uuid.UUID (can be used to un-subscribe)
func Sub(cmdC chan<- interface{}, s Subscription, fn func(interface{})) uuid.UUID {
	respC := make(chan uuid.UUID)
	cmdC <- cmdSub{
		fn:       fn,
		op:       s.Op,
		ns:       s.Ns,
		matching: s.Matching,
		name:     s.Name,
		resp:     respC,
	}
	return <-respC
}

// Unsub function remove a subscription
func Unsub(cmdC chan<- interface{}, id uuid.UUID) string {
	respC := make(chan string)
	cmdC <- cmdUnsub{
		subId: id,
		resp:  respC,
	}
	return <-respC
}
