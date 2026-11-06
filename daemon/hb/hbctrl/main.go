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

	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/daemon/hb/hbaudit"
	"github.com/opensvc/om3/v3/daemon/hbcache"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type (
	// RemoteBeating holds Remote beating stats for a remote node
	RemoteBeating struct {
		txCount     int // tx peer watcher count for a remote
		rxCount     int // rx peer watcher count for a remote
		txBeating   int
		rxBeating   int
		cancel      map[string]func()      // cancel function of hbID peer watcher for the remote
		beatingChan map[string]chan bool // beating mailbox of hbID for the remote, see sendLatest

		// watcher is the id of the hbID peer watcher for the remote: an
		// event of another watcher of hbID, deleted, is not counted.
		watcher map[string]uint64

		// beating is the rx hbIDs counted in rxBeating: an rx watcher
		// is counted once, and deleted beating, uncounted.
		beating map[string]bool
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

		// watcher is the id of the peer watcher posting the event
		watcher uint64
	}

	// EventStats is a map that holds event counters
	EventStats map[string]int

	// CmdGetEventStats is a getter of ctrl event counters
	CmdGetEventStats struct {
		result chan<- EventStats
	}

	// CmdSetAlert is a command to post new hb alert
	CmdSetAlert struct {
		HbID  string
		Alert []daemonsubsystem.Alert
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
		Desc     string
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
		hbaudit.EnableAudit(ctx, "hb.ctrl", c.log, "hb", "hb.ctrl")
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
	changed := false
	events := make(EventStats)
	remotes := make(map[string]RemoteBeating)
	heartbeat := make(map[string]daemonsubsystem.HeartbeatStream)
	pub := pubsub.PubFromContext(c.ctx)
	defer c.log.Infof("stopped: %v", events)
	updateDaemonDataHeartbeatsTicker := time.NewTicker(time.Second)
	defer updateDaemonDataHeartbeatsTicker.Stop()
	labelLocalhost := pubsub.Label{"node", hostname.Hostname()}
	labelAliveStale := pubsub.Label{"hb", "alive/stale"}

	// watcherSeq is the id of the latest peer watcher started
	var watcherSeq uint64

	// delWatcher stops the hbID watcher of peerNode. An rx watcher deleted
	// beating is not counted anymore: when it was the last beating one and
	// rx watchers of the peer remain, all stale, the peer is stale. With
	// no rx watcher left, as the restart of its only hb, nothing says the
	// peer is stale.
	delWatcher := func(hbID, peerNode string) {
		remote, ok := remotes[peerNode]
		if !ok {
			return
		}
		cancel, registered := remote.cancel[hbID]
		if !registered {
			// A stream stopped before its start added the watcher,
			// which the addition then skips.
			c.log.Infof("del watcher skipped: not registered %s -> %s", hbID, peerNode)
			return
		}
		c.log.Infof("del watcher %s -> %s", hbID, peerNode)
		cancel()
		delete(remote.cancel, hbID)
		delete(remote.beatingChan, hbID)
		delete(remote.watcher, hbID)
		isRx := strings.HasSuffix(hbID, ".rx")
		if isRx {
			remote.rxCount--
		} else {
			remote.txCount--
		}
		if remote.beating[hbID] {
			delete(remote.beating, hbID)
			remote.rxBeating--
			if remote.rxBeating == 0 && remote.rxCount > 0 {
				c.log.Infof("stale node %s: %s deleted beating, the rx watchers left are stale", peerNode, hbID)
				changed = true
				pub.Pub(&msgbus.NodeStale{Node: peerNode}, labelLocalhost)
			}
		}
		if (remote.rxCount + remote.txCount) == 0 {
			delete(remotes, peerNode)
		} else {
			remotes[peerNode] = remote
		}
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		peerDropper(c.ctx)
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
					Alerts: append([]daemonsubsystem.Alert(nil), heartbeat[key].Alerts...),
				})
			}
			hbcache.SetHeartbeats(heartbeats, changed)
			changed = false
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
				changed = true
			case CmdUnregister:
				if _, ok := heartbeat[o.ID]; ok {
					for peerNode := range remotes {
						delWatcher(o.ID, peerNode)
					}
					delete(heartbeat, o.ID)
				}
				changed = true
			case CmdSetState:
				if hbToChange, ok := heartbeat[o.ID]; ok {
					hbToChange.State = o.State
					heartbeat[o.ID] = hbToChange
				}
			case CmdSetPeerSuccess:
				if remote, ok := remotes[o.Nodename]; ok {
					k := o.HbID
					if beatC, found := remote.beatingChan[k]; found {
						sendLatest(beatC, o.Success)
					}
				}
			case CmdEvent:
				remote, ok := remotes[o.Nodename]
				if !ok || remote.watcher[o.HbID] != o.watcher {
					// Posted by a watcher deleted before the event
					// was read: its deletion counted it.
					c.log.Infof("event %s for %s from %s skipped: watcher deleted", o.Name, o.Nodename, o.HbID)
					continue
				}
				if count, ok := events[o.Name]; ok {
					events[o.Name] = count + 1
				} else {
					events[o.Name] = 1
				}
				if o.Name == evStale {
					c.log.Warnf("event %s for %s from %s", o.Name, o.Nodename, o.HbID)
					changed = true
					pub.Pub(&msgbus.HeartbeatStale{Nodename: o.Nodename, HbID: o.HbID, Time: time.Now()}, labelLocalhost, labelAliveStale)
				} else {
					c.log.Infof("event %s for %s from %s", o.Name, o.Nodename, o.HbID)
					changed = true
					pub.Pub(&msgbus.HeartbeatAlive{Nodename: o.Nodename, HbID: o.HbID, Time: time.Now()}, labelLocalhost, labelAliveStale)
				}
				if strings.HasSuffix(o.HbID, ".rx") {
					switch o.Name {
					case evBeating:
						if remote.beating[o.HbID] {
							// Counted already.
							break
						}
						if remote.rxBeating == 0 {
							c.log.Infof("beating node %s", o.Nodename)
							changed = true
							pub.Pub(&msgbus.NodeAlive{Node: o.Nodename}, labelLocalhost)
						}
						remote.beating[o.HbID] = true
						remote.rxBeating++
					case evStale:
						if !remote.beating[o.HbID] {
							// Not counted beating.
							break
						}
						if remote.rxBeating == 0 {
							panic("stale on already staled node")
						}
						delete(remote.beating, o.HbID)
						remote.rxBeating--
						if remote.rxBeating == 0 {
							c.log.Infof("stale node %s", o.Nodename)
							changed = true
							pub.Pub(&msgbus.NodeStale{Node: o.Nodename}, labelLocalhost)
						}
					}
					remotes[o.Nodename] = remote
				}
			case CmdGetEventStats:
				o.result <- events
			case GetPeerStatus:
				if foundHeartbeat, ok := heartbeat[o.HbID]; ok {
					o.result <- foundHeartbeat.Peers
				} else {
					o.result <- make(map[string]daemonsubsystem.HeartbeatStreamPeerStatus)
				}
			case CmdSetAlert:
				hbID := o.HbID
				if foundHeartbeat, ok := heartbeat[hbID]; ok {
					foundHeartbeat.Alerts = append([]daemonsubsystem.Alert(nil), o.Alert...)
					heartbeat[hbID] = foundHeartbeat
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
					remote.beatingChan = make(map[string]chan bool)
					remote.cancel = make(map[string]func())
					remote.watcher = make(map[string]uint64)
					remote.beating = make(map[string]bool)
				}
				if o.Ctx.Err() != nil {
					// The stream is already stopped: its stop sent
					// the deletion of this watcher before its start
					// sent the addition, as a restart racing the
					// start does. Registered, the watcher of the
					// stopped stream would end at once and stay in
					// the way of the watcher of the next one, as a
					// duplicate, the stream never beating again.
					c.log.Infof("watcher skipped: stream stopped %s -> %s", hbID, peerNode)
					continue
				}
				if _, registered := remote.cancel[hbID]; registered {
					c.log.Errorf("watcher skipped: duplicate %s -> %s", hbID, peerNode)
					continue
				}
				if _, ok := heartbeat[hbID]; ok {
					heartbeat[hbID].Peers[peerNode] = daemonsubsystem.HeartbeatStreamPeerStatus{
						Desc:      o.Desc,
						ChangedAt: time.Now(),
					}
				} else {
					c.log.Warnf("watcher skipped: called before register %s -> %s", hbID, peerNode)
					continue
				}
				c.log.Infof("watcher starting %s -> %s", hbID, peerNode)
				beatingC := make(chan bool, 1)
				beatingCtx, cancel := context.WithCancel(o.Ctx)
				watcherSeq++
				remote.cancel[hbID] = cancel
				remote.beatingChan[hbID] = beatingC
				remote.watcher[hbID] = watcherSeq
				if strings.HasSuffix(hbID, ".rx") {
					remote.rxCount++
				} else {
					remote.txCount++
				}
				remotes[peerNode] = remote
				c.peerWatch(beatingCtx, beatingC, o.HbID, peerNode, o.Desc, o.Timeout, watcherSeq)
			case CmdDelWatcher:
				hbID := o.HbID
				peerNode := o.Nodename
				if _, ok := heartbeat[hbID]; ok {
					delete(heartbeat[hbID].Peers, peerNode)
				}
				delWatcher(hbID, peerNode)
			}
		}
	}
}

// sendLatest puts v in the mailbox c, a channel of capacity 1 this loop alone
// sends to, replacing a value its watcher has not read yet. The watcher reads
// the values in the order they are sent, the last one always, and the loop
// never waits for it: a suspended stream reporting its peers not beating is
// not overtaken by a beat it reported before, as a value delivered by its own
// goroutine could be.
func sendLatest(c chan bool, v bool) {
	select {
	case c <- v:
		return
	default:
	}
	select {
	case <-c:
	default:
	}
	c <- v
}
