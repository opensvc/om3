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

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/daemon/daemonlogctx"
)

var (
	cmdI = make(chan interface{})
)

func Start(ctx context.Context) {
	go run(ctx)
}

func run(ctx context.Context) {
	gens := make(map[string]map[string]uint64)
	heartbeats := make([]cluster.HeartbeatStream, 0)
	log := daemonlogctx.Logger(ctx).With().Str("name", "hbcache").Logger()
	log.Debug().Msg("started")
	defer log.Debug().Msg("done")

	for {
		select {
		case <-ctx.Done():
			return
		case i := <-cmdI:
			switch cmd := i.(type) {
			case getHeartbeats:
				result := make([]cluster.HeartbeatStream, 0)
				for _, hb := range heartbeats {
					status := hb.DaemonSubsystemStatus
					status.Alerts = append([]cluster.ThreadAlert{}, hb.Alerts...)
					peers := make(map[string]cluster.HeartbeatPeerStatus)
					for node, peerStatus := range hb.Peers {
						peers[node] = peerStatus
					}
					result = append(result, cluster.HeartbeatStream{
						DaemonSubsystemStatus: status,
						Peers:                 peers,
					})
				}
				cmd.response <- result
			case dropPeer:
				delete(gens, string(cmd))
			case setHeartbeats:
				heartbeats = cmd
			default:
				log.Error().Interface("cmd", i).Msg("invalid command")
			}
		}
	}
}

// Getters

func Heartbeats() []cluster.HeartbeatStream {
	response := make(chan []cluster.HeartbeatStream)
	var i interface{} = getHeartbeats{response: response}
	cmdI <- i
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
func SetHeartbeats(hbs []cluster.HeartbeatStream) {
	var i interface{} = setHeartbeats(hbs)
	cmdI <- i
}

// commands
type (
	// getters
	getHeartbeats struct {
		response chan<- []cluster.HeartbeatStream
	}

	// setters
	dropPeer      string
	setHeartbeats []cluster.HeartbeatStream
)
