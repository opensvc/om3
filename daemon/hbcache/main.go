// Package hbcache manage heartbeat cache data for cluster
//
// This cache will be populated from:
//   - received messages from peer heratbeats
//     => peer message mode
//     => peer gens
//   - changes of local gens
//   - sent message details: 'ping', 'full' or 'size of patch queue'
//   - changes about heartbeat peers hb ping
//
// It provides the heartbeat message type to send to peers from its cached data
// It serves data for sub.hb
//
// The cache must be started with Start(ctx). It is stopped when ctx is done
package hbcache

import (
	"context"
	"sort"
	"strings"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/daemon/daemonlogctx"
	"opensvc.com/opensvc/util/hostname"
)

var (
	cmdI = make(chan interface{})
)

func Start(ctx context.Context) {
	go run(ctx)
}

func run(ctx context.Context) {
	gens := make(map[string]map[string]uint64)
	hbModes := make(map[string]string)
	heartbeats := make([]cluster.HeartbeatThreadStatus, 0)
	previousMessageType := "undef"
	localhost := hostname.Hostname()
	log := daemonlogctx.Logger(ctx).With().Str("name", "hbcache").Logger()
	log.Debug().Msg("started")
	defer log.Debug().Msg("done")

	for {
		select {
		case <-ctx.Done():
			return
		case i := <-cmdI:
			switch cmd := i.(type) {
			case getLocalGen:
				result := make(map[string]uint64)
				for node, value := range gens[localhost] {
					result[node] = value
				}
				cmd.response <- result
			case getMode:
				result := make([]cluster.HbMode, 0)
				nodes := make([]string, 0)
				for node := range hbModes {
					nodes = append(nodes, node)
				}
				sort.Strings(nodes)
				for _, node := range nodes {
					result = append(result, cluster.HbMode{
						Node: node,
						Mode: hbModes[node],
					})
				}
				cmd.response <- result
			case getMsgType:
				var messageType string
				var remoteNeedFull []string
				if len(gens) <= 1 {
					messageType = "ping"
				} else {
					for node, gen := range gens {
						if node == localhost {
							continue
						}
						if gen[localhost] == 0 {
							remoteNeedFull = append(remoteNeedFull, node)
						}
					}
					if len(remoteNeedFull) > 0 {
						messageType = "full"
					} else {
						messageType = "patch"
					}
				}
				if messageType != previousMessageType {
					if messageType == "full" {
						log.Info().Msgf("hb message type change %s -> %s local gens: %v (peers want full: %v)", previousMessageType, messageType, gens, strings.Join(remoteNeedFull, ", "))
					} else {
						log.Info().Msgf("hb message type change %s -> %s local gens: %v", previousMessageType, messageType, gens)
					}
				}
				previousMessageType = messageType
				cmd.response <- messageType
			case getHeartbeats:
				result := make([]cluster.HeartbeatThreadStatus, 0)
				for _, hb := range heartbeats {
					status := hb.ThreadStatus
					status.Alerts = append([]cluster.ThreadAlert{}, hb.Alerts...)
					peers := make(map[string]cluster.HeartbeatPeerStatus)
					for node, peerStatus := range hb.Peers {
						peers[node] = peerStatus
					}
					result = append(result, cluster.HeartbeatThreadStatus{
						ThreadStatus: status,
						Peers:        peers,
					})
				}
				cmd.response <- result
			case dropPeer:
				delete(gens, string(cmd))
				delete(hbModes, string(cmd))
			case setLocalHbMsgInfo:
				hbModes[localhost] = string(cmd)
			case setLocalGens:
				gens[localhost] = cmd
			case setFromPeerMsg:
				gens[cmd.node] = cmd.gens
				hbModes[cmd.node] = cmd.mode
			case setHeartbeats:
				heartbeats = cmd
			default:
				log.Error().Interface("cmd", i).Msg("invalid command")
			}
		}
	}
}

// Getters

// LocalGens returns the localhost gens
func LocalGens() map[string]uint64 {
	response := make(chan map[string]uint64)
	var i interface{} = getLocalGen{response: response}
	cmdI <- i
	return <-response
}

// MsgType returns the message type localhost can send to peers
//
// the returned message type value depends on cache
func MsgType() string {
	response := make(chan string)
	var i interface{} = getMsgType{response: response}
	cmdI <- i
	return <-response
}

// Modes returns []cluster.HbMode from cache
func Modes() []cluster.HbMode {
	response := make(chan []cluster.HbMode)
	var i interface{} = getMode{response: response}
	cmdI <- i
	return <-response
}

func Heartbeats() []cluster.HeartbeatThreadStatus {
	response := make(chan []cluster.HeartbeatThreadStatus)
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

// SetLocalHbMsgInfo update cache details about latest created hb message
// Examples:
//
//	SetLocalHbMsgInfo("ping")
//	SetLocalHbMsgInfo("full")
//	SetLocalHbMsgInfo("12")  // created patch message with 12 delta
func SetLocalHbMsgInfo(s string) {
	var i interface{} = setLocalHbMsgInfo(s)
	cmdI <- i
}

// SetLocalGens update cache with localhost gens
func SetLocalGens(gens map[string]uint64) {
	var i interface{} = setLocalGens(gens)
	cmdI <- i
}

// SetFromPeerMsg update cache with details from a peer message
//
// a peer message contains peer gens, and has a message type
// Example: node2 in gen 14, says it knows node1 gen 19, and propose 4 delta
//
//	SetFromPeerMsg("node2", "4", map[string]uint64{"node1": 19, "node2": 14})
func SetFromPeerMsg(peer string, mode string, peerGens map[string]uint64) {
	var i interface{} = setFromPeerMsg{node: peer, gens: peerGens, mode: mode}
	cmdI <- i
}

// SetHeartbeats updates the heartbeats status cache
//
// can be used from a heartbeat controller
func SetHeartbeats(hbs []cluster.HeartbeatThreadStatus) {
	var i interface{} = setHeartbeats(hbs)
	cmdI <- i
}

// commands
type (
	// getters

	getLocalGen struct {
		response chan<- map[string]uint64
	}
	getMsgType struct {
		response chan<- string
	}
	getMode struct {
		response chan<- []cluster.HbMode
	}
	getHeartbeats struct {
		response chan<- []cluster.HeartbeatThreadStatus
	}

	// setters
	dropPeer       string
	setFromPeerMsg struct {
		node string
		gens map[string]uint64
		mode string
	}
	setLocalGens      map[string]uint64
	setLocalHbMsgInfo string
	setHeartbeats     []cluster.HeartbeatThreadStatus
)
