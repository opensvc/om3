/*
Package hbctrl manage data and status of daemon heartbeats

It maintains the local node heartbeats status cache, this cache is published to
daemondata on a ticker interval

Example:

	c := New()
	c.Start(context.Background())

	// from another hb#2.tx routine
	// Add a watchers for hb#2.rx nodes node2 and node3
	// watchers are responsible for firing hb_stale/hb_beating event to
	// controller for hbID + remote nodename
	cmdC <- hbctrl.CmdAddWatcher{
		HbID:     "hb#2.tx",
		Nodename: "node2",
		Ctx:      ctx,
		Timeout:  r.timeout,
	}
	cmdC <- hbctrl.CmdAddWatcher{
		HbID:     "hb#2.tx",
		Nodename: "node3",
		Ctx:      ctx,
		Timeout:  r.timeout,
	}

	//set the success status of node2
	c.cmdC() <- hbctrl.CmdSetPeerSuccess{
		Nodename: "node2",
		HbID:     "hb#2.tx",
		Success:  true,
	}
*/
package hbctrl

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/opensvc/om3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/daemon/hbcache"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	// RemoteBeating holds Remote beating stats for a remote node
	RemoteBeating struct {
		txCount     int // tx peer watcher count for a remote
		rxCount     int // rx peer watcher count for a remote
		txBeating   int
		rxBeating   int
		cancel      map[string]func()      // cancel function of hbID peer watcher for the remote
		beatingChan map[string]chan<- bool // beating bool chan of hbID for the remote
	}

	// CmdRegister is the command to register a new heartbeat status
	CmdRegister struct {
		ID string // the new hb id (example: hb#1.tx)
		// Type is the hb type
		Type string
	}

	// CmdUnregister is the command to unregister a heartbeat status
	CmdUnregister struct {
		ID string // the hb id to remove (example: hb#1.tx)
	}

	// CmdSetState is the command to update a heartbeat status state
	CmdSetState struct {
		ID    string
		State string
	}

	// CmdEvent is a command to post new hb event
	CmdEvent struct {
		Name     string
		Nodename string
		HbID     string
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
		HbID     string
		Success  bool
	}

	// CmdSetPeerStatus is a command to set a hb peer HeartbeatPeerStatus for a node
	CmdSetPeerStatus struct {
		Nodename   string
		HbID       string
		PeerStatus daemonsubsystem.HeartbeatStreamPeerStatus
	}

	// CmdAddWatcher is a command to run new instance of a hb watcher for a remote
	CmdAddWatcher struct {
		HbID     string
		Nodename string
		Ctx      context.Context
		Timeout  time.Duration
	}

	// CmdDelWatcher is a command to stop one instance of a hb watcher for a remote
	CmdDelWatcher struct {
		HbID     string
		Nodename string
	}

	// GetPeerStatus is command to retrieve remote peer status for a hb
	GetPeerStatus struct {
		HbID   string
		result chan<- map[string]daemonsubsystem.HeartbeatStreamPeerStatus
	}

	// C struct holds the hb controller data
	C struct {
		cmd    chan any
		ctx    context.Context
		cancel context.CancelFunc
		log    *plog.Logger
		wg     sync.WaitGroup
	}
)

var (
	evStale   = "hb_stale"
	evBeating = "hb_beating"
)

func New() *C {
	return &C{}
}

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
func (c *C) Start(ctx context.Context) chan<- any {
	c.log = plog.NewDefaultLogger().Attr("pkg", "daemon/hbctrl").WithPrefix("daemon: hbctrl: ")
	respC := make(chan chan<- any)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		cmdC := make(chan any)
		c.ctx, c.cancel = context.WithCancel(ctx)
		c.cmd = cmdC
		respC <- cmdC
		c.run()
	}()
	return <-respC
}

func (c *C) Stop() error {
	c.cancel()
	c.wg.Wait()
	return nil
}

func (c *C) run() {
	c.log.Infof("start")
	events := make(EventStats)
	remotes := make(map[string]RemoteBeating)
	heartbeat := make(map[string]daemonsubsystem.HeartbeatStream)
	bus := pubsub.BusFromContext(c.ctx)
	defer c.log.Infof("stopped: %v", events)
	updateDaemonDataHeartbeatsTicker := time.NewTicker(time.Second)
	defer updateDaemonDataHeartbeatsTicker.Stop()

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		peerDropWorker(c.ctx)
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-updateDaemonDataHeartbeatsTicker.C:
			heartbeats := make([]daemonsubsystem.HeartbeatStream, 0)
			hbIDs := make([]string, 0)
			for hbID := range heartbeat {
				hbIDs = append(hbIDs, hbID)
			}
			sort.Strings(hbIDs)
			for _, key := range hbIDs {
				peers := make(map[string]daemonsubsystem.HeartbeatStreamPeerStatus)
				for k, v := range heartbeat[key].Peers {
					peers[k] = v
				}
				heartbeats = append(heartbeats, daemonsubsystem.HeartbeatStream{
					Status: heartbeat[key].Status,
					Type:   heartbeat[key].Type,
					Peers:  peers,
				})
			}
			hbcache.SetHeartbeats(heartbeats)
		case i := <-c.cmd:
			switch o := i.(type) {
			case CmdRegister:
				now := time.Now()
				heartbeat[o.ID] = daemonsubsystem.HeartbeatStream{
					Status: daemonsubsystem.Status{
						ID:           o.ID,
						CreatedAt:    now,
						ConfiguredAt: now,
						State:        "running",
					},
					Type:  o.Type,
					Peers: make(map[string]daemonsubsystem.HeartbeatStreamPeerStatus),
				}
			case CmdUnregister:
				if hbStatus, ok := heartbeat[o.ID]; ok {
					if strings.HasSuffix(o.ID, ".rx") {
						for peerNode, peerStatus := range hbStatus.Peers {
							if !peerStatus.IsBeating {
								continue
							}
							if peerStatus.IsBeating {
								if remote, ok := remotes[peerNode]; ok {
									remote.rxBeating--
									remotes[peerNode] = remote
								}
							}
						}
					}
					delete(heartbeat, o.ID)
				}
			case CmdSetState:
				if hbToChange, ok := heartbeat[o.ID]; ok {
					hbToChange.State = o.State
					heartbeat[o.ID] = hbToChange
				}
			case CmdSetPeerSuccess:
				if remote, ok := remotes[o.Nodename]; ok {
					k := o.HbID
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
					c.log.Warnf("event %s for %s from %s", o.Name, o.Nodename, o.HbID)
					bus.Pub(&msgbus.HbStale{Nodename: o.Nodename, HbID: o.HbID, Time: time.Now()}, label)
				} else {
					c.log.Infof("event %s for %s from %s", o.Name, o.Nodename, o.HbID)
					bus.Pub(&msgbus.HbPing{Nodename: o.Nodename, HbID: o.HbID, Time: time.Now()}, label)
				}
				if remote, ok := remotes[o.Nodename]; ok {
					if strings.HasSuffix(o.HbID, ".rx") {
						switch o.Name {
						case evBeating:
							if remote.rxBeating == 0 {
								c.log.Infof("beating node %s", o.Nodename)
								bus.Pub(&msgbus.HbNodePing{Node: o.Nodename, IsAlive: true}, pubsub.Label{"node", o.Nodename})
							}
							remote.rxBeating++
						case evStale:
							if remote.rxBeating == 0 {
								panic("stale on already staled node")
							}
							remote.rxBeating--
						}
						if remote.rxBeating == 0 {
							c.log.Infof("stale node %s", o.Nodename)
							bus.Pub(&msgbus.HbNodePing{Node: o.Nodename, IsAlive: false}, pubsub.Label{"node", o.Nodename})
						}
						remotes[o.Nodename] = remote
					}
				}
			case CmdGetEventStats:
				o.result <- events
			case GetPeerStatus:
				if foundHeartbeat, ok := heartbeat[o.HbID]; ok {
					o.result <- foundHeartbeat.Peers
				} else {
					o.result <- make(map[string]daemonsubsystem.HeartbeatStreamPeerStatus)
				}
			case CmdSetPeerStatus:
				hbID := o.HbID
				peerNode := o.Nodename
				if foundHeartbeat, ok := heartbeat[hbID]; ok {
					foundHeartbeat.Peers[peerNode] = o.PeerStatus
					heartbeat[hbID] = foundHeartbeat
				}
			case CmdAddWatcher:
				hbID := o.HbID
				peerNode := o.Nodename
				remote, ok := remotes[peerNode]
				if !ok {
					remote.beatingChan = make(map[string]chan<- bool)
					remote.cancel = make(map[string]func())
				}
				if _, registered := remote.cancel[hbID]; registered {
					c.log.Errorf("watcher skipped: duplicate %s -> %s", hbID, peerNode)
					continue
				}
				if _, ok := heartbeat[hbID]; ok {
					heartbeat[hbID].Peers[peerNode] = daemonsubsystem.HeartbeatStreamPeerStatus{}
				} else {
					c.log.Warnf("watcher skipped: called before register %s -> %s", hbID, peerNode)
					continue
				}
				c.log.Infof("watcher starting %s -> %s", hbID, peerNode)
				beatingC := make(chan bool)
				beatingCtx, cancel := context.WithCancel(o.Ctx)
				remote.cancel[hbID] = cancel
				remote.beatingChan[hbID] = beatingC
				if strings.HasSuffix(hbID, ".rx") {
					remote.rxCount++
				} else {
					remote.txCount++
				}
				remotes[peerNode] = remote
				c.peerWatch(beatingCtx, beatingC, o.HbID, peerNode, o.Timeout)
			case CmdDelWatcher:
				hbID := o.HbID
				peerNode := o.Nodename
				if _, ok := heartbeat[hbID]; ok {
					delete(heartbeat[hbID].Peers, peerNode)
				}
				if remote, ok := remotes[peerNode]; ok {
					cancel, registered := remote.cancel[hbID]
					if !registered {
						c.log.Errorf("del watcher skipped: already unregistered %s -> %s", hbID, peerNode)
						continue
					}
					c.log.Infof("del watcher %s -> %s", hbID, peerNode)
					cancel()
					delete(remote.cancel, hbID)
					if strings.HasSuffix(hbID, ".rx") {
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
