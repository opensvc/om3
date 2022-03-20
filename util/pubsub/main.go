package pubsub

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"opensvc.com/opensvc/daemon/daemonctx"
)

type (
	T struct {
		cmdC    chan interface{}
		cancel  func()
		stopped chan bool
		running bool
	}

	cmdPub struct {
		data interface{}
		resp chan<- bool
	}

	cmdSub struct {
		fn   func(interface{})
		name string
		resp chan<- uuid.UUID
	}

	cmdUnsub struct {
		subId uuid.UUID
		resp  chan<- string
	}
)

var (
	ErrorAlreadyRunning = errors.New("pubsub already running")
)

func (t *T) Stop() {
	if t.cancel == nil {
		return
	}
	t.cancel()
	t.WaitStopped()
}

func (t *T) WaitStopped() {
	if t.stopped == nil {
		return
	}
	select {
	case <-t.stopped:
	}
}

func (t *T) Run(parent context.Context, name string) (chan<- interface{}, error) {
	log := daemonctx.Logger(parent).With().Str("name", name).Logger()
	if t.running == true {
		log.Error().Err(ErrorAlreadyRunning).Msg("Run")
		return nil, ErrorAlreadyRunning
	}
	running := make(chan bool)
	cmdC := make(chan interface{})
	go func() {
		subs := make(map[uuid.UUID]func(interface{}))
		subNames := make(map[uuid.UUID]string)
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
					for _, fn := range subs {
						running := make(chan bool)
						go func() {
							running <- true
							fn(c.data)
						}()
						<-running
					}
					c.resp <- true
				case cmdSub:
					id := uuid.New()
					subs[id] = c.fn
					subNames[id] = c.name
					c.resp <- id
					log.Info().Msgf("subscribe %s", c.name)
				case cmdUnsub:
					name, ok := subNames[c.subId]
					if !ok {
						continue
					}
					delete(subs, c.subId)
					delete(subNames, c.subId)
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

func Pub(cmdC chan<- interface{}, i interface{}) {
	done := make(chan bool)
	cmdC <- cmdPub{data: i, resp: done}
	<-done
}

func Sub(cmdC chan<- interface{}, name string, fn func(interface{})) uuid.UUID {
	respC := make(chan uuid.UUID)
	cmdC <- cmdSub{
		fn:   fn,
		name: name,
		resp: respC,
	}
	return <-respC
}

func Unsub(cmdC chan<- interface{}, id uuid.UUID) string {
	respC := make(chan string)
	cmdC <- cmdUnsub{
		subId: id,
		resp:  respC,
	}
	return <-respC
}
