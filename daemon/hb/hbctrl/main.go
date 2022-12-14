/*
Package hbctrl manage data and status of daemon heartbeats

It maintains the local node heartbeats status cache, this cache is published to
daemondata on a ticker interval

Example:

	ctrl := New(context.Background())
	ctrl.start()

	// from another hb#2.tx routine
	// Add a watchers for hb#2.rx nodes node2 and node3
	// watchers are responsible for firing hb_stale/hb_beating event to
	// controller for hbId + remote nodename
	cmdC <- hbctrl.CmdAddWatcher{
		HbId:     "hb#2.tx",
		Nodename: "node2",
		Ctx:      ctx,
		Timeout:  r.timeout,
	}
	cmdC <- hbctrl.CmdAddWatcher{
		HbId:     "hb#2.tx",
		Nodename: "node3",
		Ctx:      ctx,
		Timeout:  r.timeout,
	}

	//set the success status of node2
	ctrl.cmdC() <- hbctrl.CmdSetPeerSuccess{
		Nodename: "node2",
		HbId:     "hb#2.tx",
		Success:  true,
	}
*/
package hbctrl

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/hbcache"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/pubsub"
)

type (
	// RemoteBeating holds Remote beating stats for a remote node
	RemoteBeating struct {
		txCount     int // tx peer watcher count for a remote
		rxCount     int // rx peer watcher count for a remote
		txBeating   int
		rxBeating   int
		cancel      map[string]func()      // cancel function of hbId peer watcher for the remote
		beatingChan map[string]chan<- bool // beating bool chan of hbId for the remote
	}

	// CmdRegister is the command to register a new heartbeat status
	CmdRegister struct {
		Id string // the new hb id (example: hb#1.tx)
	}

	// CmdUnregister is the command to unregister a heartbeat status
	CmdUnregister struct {
		Id string // the hb id to remove (example: hb#1.tx)
	}

	// CmdSetState is the command to update a heartbeat status state
	CmdSetState struct {
		Id    string
		State string
	}

	// CmdEvent is a command to post new hb event
	CmdEvent struct {
		Name     string
		Nodename string
		HbId     string
	}

	// EventStats is a map that holds event counters
	EventStats map[string]int

	// CmdGetEventStats is a getter of ctrl event counters
	CmdGetEventStats struct {
		result chan<- EventStats
	}

	// CmdSetPeerSuccess is a command to set a hb peer success value for a node
	CmdSetPeerSuccess struct {
		Nodename string
		HbId     string
		Success  bool
	}

	// CmdSetPeerStatus is a command to set a hb peer HeartbeatPeerStatus for a node
	CmdSetPeerStatus struct {
		Nodename   string
		HbId       string
		PeerStatus cluster.HeartbeatPeerStatus
	}

	// CmdAddWatcher is a command to run new instance of a hb watcher for a remote
	CmdAddWatcher struct {
		HbId     string
		Nodename string
		Ctx      context.Context
		Timeout  time.Duration
	}

	// CmdDelWatcher is a command to stop one instance of a hb watcher for a remote
	CmdDelWatcher struct {
		HbId     string
		Nodename string
	}

	// GetPeerStatus is command to retrieve remote peer status for a hb
	GetPeerStatus struct {
		HbId   string
		result chan<- map[string]cluster.HeartbeatPeerStatus
	}
)

type (
	// ctrl struct holds the hb controller data
	ctrl struct {
		cmd    chan any
		ctx    context.Context
		cancel context.CancelFunc
		log    zerolog.Logger
	}
)

var (
	evStale   = "hb_stale"
	evBeating = "hb_beating"
)

// Start starts hb controller goroutine, it returns its cmd chan
//
// The hb controller is responsible if the heartbeat data cache from:
// - register/unregister heartbeat tx or rx
// - addWatcher/delWatcher of a hb peer
// - setPeerSuccess
//
// # The cache is sent to daemondata on regular time interval
//
// The controller will die when ctx is done
func Start(ctx context.Context) chan any {
	c := &ctrl{
		cmd: make(chan any),
		log: log.Logger.With().Str("Name", "hbctrl").Logger(),
	}
	go c.start(ctx)
	return c.cmd
}

func (c *ctrl) start(ctx context.Context) {
	started := make(chan bool)
	go func() {
		started <- true
		c.run(ctx)
	}()
	<-started
}

func (c *ctrl) run(ctx context.Context) {
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.log.Info().Msg("start")
	events := make(EventStats)
	remotes := make(map[string]RemoteBeating)
	heartbeat := make(map[string]cluster.HeartbeatThreadStatus)
	bus := pubsub.BusFromContext(c.ctx)
	defer c.log.Info().Msgf("stopped: %v", events)
	dataCmd := daemondata.BusFromContext(ctx)
	updateDaemonDataHeartbeatsTicker := time.NewTicker(time.Second)
	defer updateDaemonDataHeartbeatsTicker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-updateDaemonDataHeartbeatsTicker.C:
			heartbeats := make([]cluster.HeartbeatThreadStatus, 0)
			hbIds := make([]string, 0)
			for hbId := range heartbeat {
				hbIds = append(hbIds, hbId)
			}
			sort.Strings(hbIds)
			for _, key := range hbIds {
				peers := make(map[string]cluster.HeartbeatPeerStatus)
				for k, v := range heartbeat[key].Peers {
					peers[k] = v
				}
				heartbeats = append(heartbeats, cluster.HeartbeatThreadStatus{
					ThreadStatus: heartbeat[key].ThreadStatus,
					Peers:        peers,
				})
			}
			hbcache.SetHeartbeats(heartbeats)
		case i := <-c.cmd:
			switch o := i.(type) {
			case CmdRegister:
				now := time.Now()
				heartbeat[o.Id] = cluster.HeartbeatThreadStatus{
					ThreadStatus: cluster.ThreadStatus{
						Id:         o.Id,
						Created:    now,
						Configured: now,
						State:      "running",
					},
					Peers: make(map[string]cluster.HeartbeatPeerStatus),
				}
			case CmdUnregister:
				if hbStatus, ok := heartbeat[o.Id]; ok {
					if strings.HasSuffix(o.Id, ".rx") {
						for peerNode, peerStatus := range hbStatus.Peers {
							if !peerStatus.Beating {
								continue
							}
							if peerStatus.Beating {
								if remote, ok := remotes[peerNode]; ok {
									remote.rxBeating--
									remotes[peerNode] = remote
								}
							}
						}
					}
					delete(heartbeat, o.Id)
				}
			case CmdSetState:
				if hbToChange, ok := heartbeat[o.Id]; ok {
					hbToChange.State = o.State
					heartbeat[o.Id] = hbToChange
				}
			case CmdSetPeerSuccess:
				if remote, ok := remotes[o.Nodename]; ok {
					k := o.HbId
					if beatC, found := remote.beatingChan[k]; found {
						go func() {
							beatC <- o.Success
						}()
					}
				}
			case CmdEvent:
				if count, ok := events[o.Name]; ok {
					events[o.Name] = count + 1
				} else {
					events[o.Name] = 1
				}
				label := pubsub.Label{"hb", "ping/stale"}
				if o.Name == evStale {
					c.log.Warn().Msgf("event %s for %s from %s", o.Name, o.Nodename, o.HbId)
					bus.Pub(msgbus.HbStale{Nodename: o.Nodename, HbId: o.HbId, Time: time.Now()}, label)
				} else {
					c.log.Info().Msgf("event %s for %s from %s", o.Name, o.Nodename, o.HbId)
					bus.Pub(msgbus.HbPing{Nodename: o.Nodename, HbId: o.HbId, Time: time.Now()}, label)
				}
				if remote, ok := remotes[o.Nodename]; ok {
					if strings.HasSuffix(o.HbId, ".rx") {
						switch o.Name {
						case evBeating:
							if remote.rxBeating == 0 {
								c.log.Info().Msgf("beating node %s", o.Nodename)
								if err := daemondata.SetHeartbeatPing(dataCmd, o.Nodename, true); err != nil {
									c.log.Error().Err(err).Msg("set heartbeat ping on alive node")
								}
							}
							remote.rxBeating++
						case evStale:
							if remote.rxBeating == 0 {
								panic("stale on already staled node")
							}
							remote.rxBeating--
						}
						if remote.rxBeating == 0 {
							c.log.Info().Msgf("stale node %s", o.Nodename)
							if err := daemondata.SetHeartbeatPing(dataCmd, o.Nodename, false); err != nil {
								c.log.Error().Err(err).Msg("set heartbeat ping on dead node")
							}
						}
						remotes[o.Nodename] = remote
					}
				}
			case CmdGetEventStats:
				o.result <- events
			case GetPeerStatus:
				if foundHeartbeat, ok := heartbeat[o.HbId]; ok {
					o.result <- foundHeartbeat.Peers
				} else {
					o.result <- make(map[string]cluster.HeartbeatPeerStatus)
				}
			case CmdSetPeerStatus:
				hbId := o.HbId
				peerNode := o.Nodename
				if foundHeartbeat, ok := heartbeat[hbId]; ok {
					foundHeartbeat.Peers[peerNode] = o.PeerStatus
					heartbeat[hbId] = foundHeartbeat
				}
			case CmdAddWatcher:
				hbId := o.HbId
				peerNode := o.Nodename
				remote, ok := remotes[peerNode]
				if !ok {
					remote.beatingChan = make(map[string]chan<- bool)
					remote.cancel = make(map[string]func())
				}
				if _, registered := remote.cancel[hbId]; registered {
					c.log.Error().Msgf("CmdAddWatcher already registered watcher %s %s", hbId, peerNode)
					continue
				}
				if _, ok := heartbeat[hbId]; ok {
					heartbeat[hbId].Peers[peerNode] = cluster.HeartbeatPeerStatus{}
				} else {
					c.log.Warn().Msgf("CmdAddWatcher %s %s called before CmdRegister", hbId, peerNode)
					continue
				}
				beatingC := make(chan bool)
				beatingCtx, cancel := context.WithCancel(o.Ctx)
				remote.cancel[hbId] = cancel
				remote.beatingChan[hbId] = beatingC
				c.log.Info().Msgf("register watcher %s for %s", peerNode, hbId)
				if strings.HasSuffix(hbId, ".rx") {
					remote.rxCount++
				} else {
					remote.txCount++
				}
				remotes[peerNode] = remote
				c.peerWatch(beatingCtx, beatingC, o.HbId, peerNode, o.Timeout)
			case CmdDelWatcher:
				hbId := o.HbId
				peerNode := o.Nodename
				if _, ok := heartbeat[hbId]; ok {
					delete(heartbeat[hbId].Peers, peerNode)
				}
				if remote, ok := remotes[peerNode]; ok {
					cancel, registered := remote.cancel[hbId]
					if !registered {
						c.log.Error().Msgf("CmdDelWatcher already unregistered watcher %s %s", hbId, peerNode)
						continue
					}
					c.log.Info().Msgf("unregister watcher %s %s", hbId, peerNode)
					cancel()
					delete(remote.cancel, hbId)
					if strings.HasSuffix(hbId, ".rx") {
						remote.rxCount--
					} else {
						remote.txCount--
					}
					if (remote.rxCount + remote.txCount) == 0 {
						delete(remotes, peerNode)
					} else {
						remotes[peerNode] = remote
					}
				}
			}
		}
	}
}

func GetEventStats(c chan<- any) EventStats {
	result := make(chan EventStats)
	c <- CmdGetEventStats{result: result}
	return <-result
}
