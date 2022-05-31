// Package pubsub implements simple pub-sub
//
// Example:
//	  import (
//    	"context"
//    	"fmt"
//
//    	ps "opensvc.com/opensvc/util/pubsub"
//    )
//
//    func main() {
//    	const (
//    		NsNum1 = ps.NsAll + 1 + iota
//    		NsNum2
//    	)
//
//    	// Start the pub-sub
//    	pubSub1 := ps.T{}
//    	c, err := pubSub1.Start(context.Background(), "pub-sub example")
//    	if err != nil {
//    		return
//    	}
//    	defer pubSub1.Stop()
//
//    	// Prepare a new subscription details
//    	subOnCreate := ps.Subscription{
//    		Ns:       NsNum1,
//    		Op:       ps.OpCreate,
//    		Matching: "idA",
//    		Name:     "subscription example",
//    	}
//
//    	// register the subscription
//    	sub1Id := ps.Sub(c, subOnCreate, func(i interface{}) {
//    		fmt.Printf("detected from subscription 1: value '%s' has been published with operation 'OpCreate' on id 'IdA' in name space 'NsNum1'\n", i)
//    	})
//    	defer ps.Unsub(c, sub1Id)
//
//    	// register another subscription that watch all namespaces/operations/ids
//    	defer ps.Unsub(
//    		c,
//    		ps.Sub(c,
//    			ps.Subscription{Name: "watch all"},
//    			func(i interface{}) {
//    				fmt.Printf("detected from subscription 2: value '%s' have been published\n", i)
//    			}))
//
//    	// publish a create operation of "something" on namespace NsNum1
//    	ps.Pub(c, ps.Publication{
//    		Ns:    NsNum1,
//    		Op:    ps.OpCreate,
//    		Id:    "idA",
//    		Value: "foo bar",
//    	})
//
//    	// publish a Update operation of "a value" on namespace NsNum2
//    	ps.Pub(c, ps.Publication{
//    		Ns:    NsNum2,
//    		Op:    ps.OpUpdate,
//    		Id:    "idXXXX",
//    		Value: "a value",
//    	})
//    }
//

package pubsub

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"opensvc.com/opensvc/daemon/daemonctx"
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
	// T struct holds internal data for a pub-sub
	T struct {
		cmdC    chan interface{}
		cancel  func()
		stopped chan bool
		running bool
	}

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

var (
	ErrorAlreadyRunning = errors.New("pub sub already running")
)

// Stop function stops the pub-sub worker
func (t *T) Stop() {
	if t.cancel == nil {
		return
	}
	t.cancel()
	t.waitStopped()
}

// Start function starts the pub-sub
func (t *T) Start(parent context.Context, name string) (chan<- interface{}, error) {
	log := daemonctx.Logger(parent).With().Str("name", name).Logger()
	if t.running == true {
		log.Error().Err(ErrorAlreadyRunning).Msg("Start")
		return nil, ErrorAlreadyRunning
	}
	running := make(chan bool)
	cmdC := make(chan interface{})
	go func() {
		subs := make(map[uuid.UUID]func(interface{}))
		subNames := make(map[uuid.UUID]string)
		subNs := make(map[uuid.UUID]uint)
		subOps := make(map[uuid.UUID]uint)
		subMatching := make(map[uuid.UUID]string)
		ctx, cancel := context.WithCancel(parent)
		t.stopped = make(chan bool)
		t.cancel = cancel
		defer func() {
			log.Info().Msg("stopping")
			cancel()
			t.cancel = nil
			t.running = false
			close(t.stopped)
			log.Info().Msg("stopped")
		}()
		running <- true
		for {
			select {
			case <-ctx.Done():
				return
			case cmd := <-cmdC:
				switch c := cmd.(type) {
				case cmdPub:
					for id, fn := range subs {
						if subNs[id] != NsAll && subNs[id] != c.ns {
							continue
						}
						if subOps[id] != OpAll && subOps[id] != c.op {
							continue
						}
						if len(subMatching[id]) != 0 && subMatching[id] != c.id {
							continue
						}
						running := make(chan bool)
						goFunc := fn
						go func() {
							running <- true
							goFunc(c.data)
						}()
						<-running
					}
					c.resp <- true
				case cmdSub:
					id := uuid.New()
					subs[id] = c.fn
					subNames[id] = c.name
					subNs[id] = c.ns
					subOps[id] = c.op
					subMatching[id] = c.matching
					c.resp <- id
					log.Info().Msgf("subscribe %s", c.name)
				case cmdUnsub:
					name, ok := subNames[c.subId]
					if !ok {
						continue
					}
					delete(subs, c.subId)
					delete(subNames, c.subId)
					delete(subNs, c.subId)
					delete(subOps, c.subId)
					c.resp <- name
					log.Info().Msgf("unsubscribe %s", name)
				}
			}
		}
	}()
	t.running = <-running
	log.Info().Msg("running")
	return cmdC, nil
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

func (t *T) waitStopped() {
	if t.stopped == nil {
		return
	}
	select {
	case <-t.stopped:
	}
}
