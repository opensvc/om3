package nmon

import (
	"slices"
	"time"

	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/errcontext"
)

type (
	// networkSetupResult is the end of a network setup run off the loop.
	networkSetupResult struct {
		names   []string
		err     error
		sigs    map[string]string
		waiters []errcontext.ErrCloseSender
	}
)

var (
	// networkSetup sets up the networks named, all of them when none is
	// named. Tests replace it.
	networkSetup = func(n *object.Node, names ...string) error {
		return network.Setup(n, names...)
	}

	// networkSetupWarnAfter is the time a network setup runs before it is
	// warned about: a setup hung, as one waiting for a name of a peer to
	// resolve, holds the node in rejoin at the daemon start.
	networkSetupWarnAfter = 30 * time.Second
)

// requestNetworkSetup sets up the networks named, all of them when names is
// empty, off the loop: a setup, writing the cni configurations, the bridges,
// the routes and the firewall rules of the networks, takes seconds, during
// which the monitor read no request, as a freeze or the end of a split.
//
// One setup runs at a time, as two would write the same firewall ruleset. A
// setup asked while one runs is merged into the one run when it ends, the
// setup of all the networks taking in the ones of named networks. waiter,
// when not nil, is sent the error of the setup run once the request is
// merged.
func (t *Manager) requestNetworkSetup(names []string, waiter errcontext.ErrCloseSender) {
	if t.networkSetupRunning {
		switch {
		case t.networkSetupPending == nil:
			pending := slices.Clone(names)
			if pending == nil {
				pending = []string{}
			}
			t.networkSetupPending = &pending
		case len(*t.networkSetupPending) == 0, len(names) == 0:
			// all the networks
			*t.networkSetupPending = []string{}
		default:
			for _, name := range names {
				if !slices.Contains(*t.networkSetupPending, name) {
					*t.networkSetupPending = append(*t.networkSetupPending, name)
				}
			}
		}
		if waiter != nil {
			t.networkSetupPendingWaiters = append(t.networkSetupPendingWaiters, waiter)
		}
		return
	}
	var waiters []errcontext.ErrCloseSender
	if waiter != nil {
		waiters = append(waiters, waiter)
	}
	t.startNetworkSetup(names, waiters)
}

func (t *Manager) startNetworkSetup(names []string, waiters []errcontext.ErrCloseSender) {
	t.networkSetupRunning = true
	ctx, log, last := t.ctx, t.log, t.networkPeersSigs
	go func() {
		begin := time.Now()
		warn := time.AfterFunc(networkSetupWarnAfter, func() {
			log.Warnf("network setup %v runs for %s", names, networkSetupWarnAfter)
		})
		r := networkSetupResult{names: names, waiters: waiters}
		n, err := object.NewNode(object.WithLogger(log))
		if err == nil {
			r.err = networkSetup(n, names...)
		} else {
			r.err = err
		}
		r.sigs = networkPeersSigs(n, last, log)
		if !warn.Stop() {
			log.Infof("network setup %v done in %s", names, time.Since(begin))
		}
		select {
		case t.networkSetupC <- r:
		case <-ctx.Done():
		}
	}()
}

// onNetworkSetupDone applies the end of a network setup: the peers of the
// networks changed are published, the requests waiting for it answered, the
// node goes idle if it waited for its first setup, and the setup asked
// meanwhile runs.
func (t *Manager) onNetworkSetupDone(r networkSetupResult) {
	t.networkSetupRunning = false
	if r.err != nil {
		t.log.Infof("network setup %v: %s", r.names, r.err)
	}
	t.publishNetworkPeersUpdated(r.sigs)
	for _, waiter := range r.waiters {
		waiter.Send(r.err)
	}
	if !t.networksReady {
		t.networksReady = true
		if t.idleWanted {
			t.idleWanted = false
			if t.state.State == node.MonitorStateRejoin {
				t.log.Infof("the networks are set up: transition to idle")
				t.transitionTo(node.MonitorStateIdle)
			}
		}
	}
	if t.networkSetupPending != nil {
		names, waiters := *t.networkSetupPending, t.networkSetupPendingWaiters
		t.networkSetupPending, t.networkSetupPendingWaiters = nil, nil
		t.startNetworkSetup(names, waiters)
	}
}

// rejoined ends the rejoin of the node: it goes idle, and its instances
// start, once the networks are set up.
func (t *Manager) rejoined() {
	if t.networksReady {
		t.transitionTo(node.MonitorStateIdle)
		return
	}
	if !t.idleWanted {
		t.log.Infof("wait for the networks to be set up to transition to idle")
	}
	t.idleWanted = true
}

// onNetworkSetupRequest sets up the networks a request names, as the api
// asks, its error sent to the request once the setup ran.
func (t *Manager) onNetworkSetupRequest(c *msgbus.NetworkSetupRequest) {
	if c.Node != t.localhost {
		return
	}
	t.log.Infof("set up the networks %v on request", c.Names)
	t.requestNetworkSetup(c.Names, c.Err)
}
