/*
	Package hbctrl manage data from hb thew a Cmd chan

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
	"encoding/json"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/pubsub"
)

type (
	// T struct holds the hb controller data
	T struct {
		cmd    chan interface{}
		ctx    context.Context
		cancel context.CancelFunc
		log    zerolog.Logger
	}

	// RemoteBeating holds Remote beating stats for a remote node
	RemoteBeating struct {
		txCount     int
		rxCount     int
		txBeating   int
		rxBeating   int
		cancel      map[string]func()
		beatingChan map[string]chan<- bool
	}

	// CmdEvent is a command to post new hb event
	CmdEvent struct {
		Name     string
		Nodename string
		HbId     string
	}

	// EventStats is a map that holds event counters
	EventStats map[string]int

	// GetEventStats is a getter of ctrl event counters
	GetEventStats struct {
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

// New return a new hb controller
func New() *T {
	return &T{
		cmd: make(chan interface{}),
		log: log.Logger.With().Str("Name", "hbctrl").Logger(),
	}
}

// Stop function cancel a running T controller
func (t *T) Stop() {
	if t.cancel != nil {
		t.cancel()
	}
}

// Start Watch and respond on Cmd chan, until a Stop() call
func (t *T) Start(ctx context.Context) {
	t.ctx, t.cancel = context.WithCancel(ctx)
	t.log.Info().Msg("start")
	events := make(EventStats)
	remotes := make(map[string]RemoteBeating)
	hbBeatings := make(map[string]map[string]cluster.HeartbeatPeerStatus)
	bus := pubsub.BusFromContext(t.ctx)
	defer t.log.Info().Msgf("stopped: %v", events)
	for {
		select {
		case <-t.ctx.Done():
			return
		case i := <-t.cmd:
			switch o := i.(type) {
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
				var data json.RawMessage
				data = []byte("\"" + o.Name + " " + o.Nodename + " detected by " + o.HbId + "\"")
				msgbus.PubEvent(bus, event.Event{
					Kind: o.Name,
					ID:   0,
					Time: time.Now(),
					Data: &data,
				})
				t.log.Info().Msgf("Received event %s for %s from %s", o.Name, o.Nodename, o.HbId)
			case GetEventStats:
				o.result <- events
			case GetPeerStatus:
				hbBeating, ok := hbBeatings[o.HbId]
				if !ok {
					o.result <- map[string]cluster.HeartbeatPeerStatus{}
				} else {
					o.result <- hbBeating
				}
			case CmdSetPeerStatus:
				key := o.HbId
				hbBeating, ok := hbBeatings[key]
				if !ok {
					t.log.Debug().Msgf("skip CmdSetPeerStatus for %s (no hbBeatings[%s])", o.Nodename, key)
					continue
				}
				hbBeating[o.Nodename] = o.PeerStatus
			case CmdAddWatcher:
				k := o.HbId
				remote, ok := remotes[o.Nodename]
				if !ok {
					hbBeatings[k] = make(map[string]cluster.HeartbeatPeerStatus)
					remote.beatingChan = make(map[string]chan<- bool)
					remote.cancel = make(map[string]func())
				}
				if _, registered := remote.cancel[k]; registered {
					t.log.Error().Msgf("already registered watcher %s %s", k, o.Nodename)
					continue
				} else {
					hbBeatings[k] = make(map[string]cluster.HeartbeatPeerStatus)
				}
				beatingC := make(chan bool)
				beatingCtx, cancel := context.WithCancel(o.Ctx)
				remote.cancel[k] = cancel
				remote.beatingChan[k] = beatingC
				t.log.Info().Msgf("register watcher %s for %s", o.Nodename, k)
				remotes[o.Nodename] = remote
				if strings.HasSuffix(o.HbId, ".rx") {
					remote.rxCount++
				} else {
					remote.txCount++
				}
				t.peerWatch(beatingCtx, beatingC, o.HbId, o.Nodename, o.Timeout)
			case CmdDelWatcher:
				remote, ok := remotes[o.Nodename]
				if ok {
					k := o.HbId
					cancel, registered := remote.cancel[k]
					if !registered {
						t.log.Error().Msgf("already unregistered watcher %s %s", o.HbId, o.Nodename)
						continue
					}
					t.log.Info().Msgf("unregister watcher %s %s", o.HbId, o.Nodename)
					cancel()
					if strings.HasSuffix(o.HbId, ".rx") {
						remote.rxCount--
					} else {
						remote.txCount--
					}
					delete(hbBeatings, k)
					if (remote.rxCount + remote.txCount) == 0 {
						delete(remotes, o.Nodename)
					}
				}
			}
		}
	}
}

// Cmd returns the T Cmd chan to submit new command to ctrl
func (t *T) Cmd() chan<- interface{} {
	return t.cmd
}

func (t *T) GetEventStats() EventStats {
	result := make(chan EventStats)
	t.cmd <- GetEventStats{result: result}
	return <-result
}
