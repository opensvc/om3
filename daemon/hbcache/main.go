// Package hbcache manage []cluster.HeartbeatStream cache localnode
//
// This cache will be populated from:
//   - heartbeat status
//
// # It provides the heartbeat for sub.hb.heartbeat
//
// The cache must be started with Start(ctx). It is stopped when ctx is done
package hbcache

import (
	"context"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/daemon/draincommand"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	T struct {
		drainDuration time.Duration
		ctx           context.Context
		cancel        context.CancelFunc
		wg            sync.WaitGroup
	}
)

var (
	cmdI = make(chan interface{})
)

func New(drainDuration time.Duration) *T {
	return &T{drainDuration: drainDuration}
}

func (t *T) Start(ctx context.Context) error {
	err := make(chan error)
	t.wg.Add(1)
	go func(err chan<- error) {
		defer t.wg.Done()
		err <- nil
		t.run(ctx)
	}(err)
	return <-err
}

func (t *T) Stop() error {
	t.cancel()
	t.wg.Wait()
	return nil
}

func (t *T) run(ctx context.Context) {
	changed := false
	gens := make(map[string]node.Gen)
	heartbeats := make([]daemonsubsystem.HeartbeatStream, 0)
	log := plog.NewDefaultLogger().WithPrefix("daemon: hbcache: ").Attr("pkg", "daemon/hbcache")
	log.Tracef("started")
	defer log.Tracef("done")
	defer draincommand.Do(cmdI, t.drainDuration)
	t.ctx, t.cancel = context.WithCancel(ctx)
	for {
		select {
		case <-t.ctx.Done():
			return
		case i := <-cmdI:
			switch cmd := i.(type) {
			case getHeartbeats:
				result := make([]daemonsubsystem.HeartbeatStream, 0, len(heartbeats))
				for _, hb := range heartbeats {
					result = append(result, *hb.DeepCopy())
				}
				cmd.errC <- nil
				cmd.response <- result
				cmd.changed <- changed
				changed = false
			case dropPeer:
				delete(gens, string(cmd))
			case setHeartbeats:
				heartbeats = cmd.hbs
				if cmd.changed {
					changed = true
				}
			default:
				log.Errorf("invalid command: %i", i)
			}
		}
	}
}

// Getters

// Heartbeats retrieves the current list of heartbeat streams and their status,
// returning a boolean indicating changes since last cammand call.
func Heartbeats() ([]daemonsubsystem.HeartbeatStream, bool) {
	err := make(chan error, 1)
	response := make(chan []daemonsubsystem.HeartbeatStream)
	changed := make(chan bool)
	cmd := getHeartbeats{
		errC:     err,
		response: response,
		changed:  changed,
	}
	var i interface{} = cmd
	cmdI <- i
	if <-err != nil {
		return nil, false
	}
	return <-response, <-changed
}

// Setters

// DropPeer drop a node from cache
func DropPeer(peer string) {
	var i interface{} = dropPeer(peer)
	cmdI <- i
}

// SetHeartbeats updates the heartbeats status cache
//
// can be used from a heartbeat controller
func SetHeartbeats(hbs []daemonsubsystem.HeartbeatStream, changed bool) {
	var i interface{} = setHeartbeats{hbs: hbs, changed: changed}
	cmdI <- i
}

// commands
type (
	errC draincommand.ErrC

	// getters
	getHeartbeats struct {
		errC
		response chan<- []daemonsubsystem.HeartbeatStream
		changed  chan<- bool
	}

	// setters
	dropPeer      string
	setHeartbeats struct {
		hbs     []daemonsubsystem.HeartbeatStream
		changed bool
	}
)
