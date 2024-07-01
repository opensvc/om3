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

	"github.com/opensvc/om3/daemon/draincommand"
	"github.com/opensvc/om3/daemon/dsubsystem"
	"github.com/opensvc/om3/util/plog"
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
	gens := make(map[string]map[string]uint64)
	heartbeats := make([]dsubsystem.HeartbeatStream, 0)
	log := plog.NewDefaultLogger().WithPrefix("daemon: hbcache: ").Attr("pkg", "daemon/hbcache")
	log.Debugf("started")
	defer log.Debugf("done")
	defer draincommand.Do(cmdI, t.drainDuration)
	t.ctx, t.cancel = context.WithCancel(ctx)
	for {
		select {
		case <-t.ctx.Done():
			return
		case i := <-cmdI:
			switch cmd := i.(type) {
			case getHeartbeats:
				result := make([]dsubsystem.HeartbeatStream, 0)
				for _, hb := range heartbeats {
					status := hb.DaemonSubsystemStatus
					status.Alerts = append([]dsubsystem.ThreadAlert{}, hb.Alerts...)
					peers := make(map[string]dsubsystem.HeartbeatPeerStatus)
					for node, peerStatus := range hb.Peers {
						peers[node] = peerStatus
					}
					result = append(result, dsubsystem.HeartbeatStream{
						DaemonSubsystemStatus: status,
						Peers:                 peers,
						Type:                  hb.Type,
					})
				}
				cmd.errC <- nil
				cmd.response <- result
			case dropPeer:
				delete(gens, string(cmd))
			case setHeartbeats:
				heartbeats = cmd
			default:
				log.Errorf("invalid command: %i", i)
			}
		}
	}
}

// Getters

func Heartbeats() []dsubsystem.HeartbeatStream {
	err := make(chan error, 1)
	response := make(chan []dsubsystem.HeartbeatStream)
	cmd := getHeartbeats{
		errC:     err,
		response: response,
	}
	var i interface{} = cmd
	cmdI <- i
	if <-err != nil {
		return nil
	}
	return <-response
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
func SetHeartbeats(hbs []dsubsystem.HeartbeatStream) {
	var i interface{} = setHeartbeats(hbs)
	cmdI <- i
}

// commands
type (
	errC draincommand.ErrC

	// getters
	getHeartbeats struct {
		errC
		response chan<- []dsubsystem.HeartbeatStream
	}

	// setters
	dropPeer      string
	setHeartbeats []dsubsystem.HeartbeatStream
)
