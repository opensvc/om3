package hbctrl

import (
	"context"
	"time"

	"github.com/opensvc/om3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/util/plog"
)

var (
	// pubDelay is the delayed duration after a change to re-read current
	// beating. It is useful to reduce beating event published on fast beating changes
	// that may occur on ctrl-Z daemon.
	// Example:
	// t0: initial beating: true
	// t1: evStale => beating false
	// if t1 + pubDelay < t2: read current beating (false) => publish stale event
	// t2: evBeating => beating true
	// if t1 + pubDelay > t2: read current beating (true) => no event
	pubDelay = 200 * time.Millisecond
)

// peerWatch starts a new peer watcher of nodename for hbID
// when beating state change a hb_beating or hb_stale event is fired
// Once beating, a hb_stale event is fired if no beating are received after timeout
func (c *C) peerWatch(ctx context.Context, beatingC chan bool, HbID, nodename, desc string, timeout time.Duration) {
	peer := daemonsubsystem.HeartbeatStreamPeerStatus{
		Desc: desc,
	}
	started := make(chan bool)
	go func() {
		// changes tracks beating value changes
		var changes bool

		// beatingOnLastPub is the latest peer.Beating value changed
		var beatingOnLastPub bool

		// pubTicker the interval ticker to verify if peer.Beating != lastBeating
		pubTicker := time.NewTicker(pubDelay)
		defer pubTicker.Stop()
		pubTicker.Stop()

		// staleTicker is the ticker to watch beating==true not refreshed since timeout
		// Reset when receive a beating true
		// Stop when receive a beating false
		staleTicker := time.NewTicker(timeout)
		staleTicker.Stop()
		defer staleTicker.Stop()
		log := plog.NewDefaultLogger().Attr("pkg", "daemon/hbctrl:peerWatch").Attr("hb_peer_watch", HbID+"-"+nodename).WithPrefix("daemon: hbctrl: peer watcher: " + HbID + "-" + nodename + ": ")
		log.Infof("started")
		started <- true
		setBeating := func(v bool) {
			peer.IsBeating = v
			changes = true
			pubTicker.Reset(pubDelay)
		}
		for {
			select {
			case <-ctx.Done():
				log.Infof("done")
				return
			case <-c.ctx.Done():
				log.Infof("done (from ctrl done)")
				return
			case beating := <-beatingC:
				switch {
				case beating && peer.IsBeating:
					// continue beating (normal situation)
					staleTicker.Reset(timeout)
					peer.LastAt = time.Now()
				case beating && !peer.IsBeating:
					// resume beating
					setBeating(true)
					staleTicker.Reset(timeout)
					peer.LastAt = time.Now()
				case !beating && peer.IsBeating:
					// stop beating
					setBeating(false)
					staleTicker.Stop()
				}
			case <-pubTicker.C:
				pubTicker.Stop()
				if changes {
					changes = false
					if beatingOnLastPub != peer.IsBeating {
						evName := evBeating
						if !peer.IsBeating {
							evName = evStale
						}
						c.cmd <- CmdEvent{
							Name:     evName,
							Nodename: nodename,
							HbID:     HbID,
						}
						c.cmd <- CmdSetPeerStatus{
							Nodename:   nodename,
							HbID:       HbID,
							PeerStatus: peer,
						}
						beatingOnLastPub = peer.IsBeating
					}
				}
			case <-staleTicker.C:
				if peer.IsBeating {
					setBeating(false)
				}
				staleTicker.Stop()
			}
		}
	}()
	<-started
}
