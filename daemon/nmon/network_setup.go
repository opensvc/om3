package nmon

import (
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/errcontext"
)

type (
	// networkSetupResult is the end of a network setup run off the loop.
	// networkSetupJob is a network setup to run: the networks named, all
	// of them when names is empty, the requests waiting for its end, and
	// the session of the client following its logs, if any.
	networkSetupJob struct {
		names     []string
		waiters   []errcontext.ErrCloseSender
		sessionID uuid.UUID
	}

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
// setup asked while one runs is merged into a setup queued, run when the
// running one ends, the setup of all the networks taking in the ones of
// named networks. waiter, when not nil, is sent the error of the setup run
// once the request is merged.
//
// sessionID, when not nil, is the session of the client asking, which
// follows the logs of the setup by it: a run logs with one session id, so a
// request of a session is merged only into a setup queued for no session or
// for the same one, the others queued after it. The requests of the daemon,
// with none, merge into any.
func (t *Manager) requestNetworkSetup(names []string, waiter errcontext.ErrCloseSender, sessionID uuid.UUID) {
	if t.networkSetupRunning {
		t.queueNetworkSetup(names, waiter, sessionID)
		return
	}
	job := &networkSetupJob{names: names, sessionID: sessionID}
	if waiter != nil {
		job.waiters = append(job.waiters, waiter)
	}
	t.startNetworkSetup(job)
}

// queueNetworkSetup merges a request into the first setup queued it can be
// merged into, or queues a setup for it.
func (t *Manager) queueNetworkSetup(names []string, waiter errcontext.ErrCloseSender, sessionID uuid.UUID) {
	var job *networkSetupJob
	for _, queued := range t.networkSetupQueue {
		if sessionID == uuid.Nil || queued.sessionID == uuid.Nil || queued.sessionID == sessionID {
			job = queued
			break
		}
	}
	if job == nil {
		job = &networkSetupJob{names: []string{}}
		if len(names) > 0 {
			job.names = slices.Clone(names)
		}
		t.networkSetupQueue = append(t.networkSetupQueue, job)
	} else {
		job.merge(names)
	}
	if job.sessionID == uuid.Nil {
		job.sessionID = sessionID
	}
	if waiter != nil {
		job.waiters = append(job.waiters, waiter)
	}
}

// merge takes in the networks named, all of them when names is empty: the
// setup of all the networks takes in the ones of named networks.
func (j *networkSetupJob) merge(names []string) {
	switch {
	case len(j.names) == 0, len(names) == 0:
		j.names = []string{}
	default:
		for _, name := range names {
			if !slices.Contains(j.names, name) {
				j.names = append(j.names, name)
			}
		}
	}
}

func (t *Manager) startNetworkSetup(job *networkSetupJob) {
	t.networkSetupRunning = true
	ctx, log, last := t.ctx, t.log, t.networkPeersSigs
	names := job.names
	if job.sessionID != uuid.Nil {
		// The client of the session follows the logs of the setup.
		log = log.Attr("session_id", job.sessionID.String())
	}
	// The monitor stop waits for the setup: the next daemon would set the
	// networks up while this one still changes them.
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		begin := time.Now()
		warn := time.AfterFunc(networkSetupWarnAfter, func() {
			log.Warnf("network setup %v runs for %s", names, networkSetupWarnAfter)
		})
		r := networkSetupResult{names: names, waiters: job.waiters}
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
	if len(t.networkSetupQueue) > 0 {
		job := t.networkSetupQueue[0]
		t.networkSetupQueue = t.networkSetupQueue[1:]
		t.startNetworkSetup(job)
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
	t.requestNetworkSetup(c.Names, c.Err, c.SessionID)
}
