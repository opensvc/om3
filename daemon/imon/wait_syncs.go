package imon

import (
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/status"
)

// defaultWaitSyncsTimeout is how long a stop waits for the syncs running on
// the instance to end, before it gives up, when the configuration of the
// instance does not say: DEFAULT.wait_syncs_timeout.
const defaultWaitSyncsTimeout = 10 * time.Minute

func (t *Manager) waitSyncsTimeout() time.Duration {
	if t.instConfig.ActorConfig != nil && t.instConfig.ActorConfig.WaitSyncsTimeout > 0 {
		return t.instConfig.ActorConfig.WaitSyncsTimeout
	}
	return defaultWaitSyncsTimeout
}

// interruptSyncsAsked says the orchestration asks the stop to interrupt the
// syncs running, instead of waiting for them: a stop or a switch with
// --interrupt-syncs.
func (t *Manager) interruptSyncsAsked() bool {
	switch options := t.state.GlobalExpectOptions.(type) {
	case instance.MonitorGlobalExpectOptionsStopped:
		return options.InterruptSyncs
	case instance.MonitorGlobalExpectOptionsPlacedAt:
		return options.InterruptSyncs
	default:
		return false
	}
}

// runningSyncs is the sync resources of the local instance running now, as
// their run files say.
func (t *Manager) runningSyncs() []string {
	var l []string
	for _, r := range t.instStatus[t.localhost].Running {
		if id, err := resourceid.Parse(r.RID); err == nil && id.DriverGroup() == driver.GroupSync {
			l = append(l, r.RID)
		}
	}
	return l
}

// setWaitSyncs holds a stop while a sync runs on the instance, and reports
// whether it does.
//
// A sync running holds the object lock the stop needs, and sends data to the
// peers: stopped under it, or failing over from it, the peers would take over
// a copy the sync was writing. The stop waits for the syncs to end, in the wait
// syncs state, for DEFAULT.wait_syncs_timeout at most. Past it, the stop fails, and the
// data the peers hold is left as the syncs make it.
func (t *Manager) setWaitSyncs() bool {
	if t.interruptSyncsAsked() {
		// The stop ends the syncs itself, instead of waiting for them.
		if t.state.State == instance.MonitorStateWaitSyncs {
			t.waitSyncsSince = time.Time{}
			t.waitSyncsTimer.Stop()
			t.transitionTo(instance.MonitorStateIdle)
		}
		return false
	}
	rids := t.runningSyncs()
	if len(rids) == 0 {
		if t.state.State == instance.MonitorStateWaitSyncs {
			t.log.Infof("no more syncs to wait")
			t.waitSyncsSince = time.Time{}
			t.waitSyncsTimer.Stop()
			t.transitionTo(instance.MonitorStateIdle)
		}
		return false
	}
	if t.state.State != instance.MonitorStateWaitSyncs {
		t.log.Infof("wait syncs %s to end before stopping, %s at most", strings.Join(rids, ","), t.waitSyncsTimeout())
		t.waitSyncsSince = time.Now()
		t.waitSyncsTimer.Reset(t.waitSyncsTimeout())
		t.transitionTo(instance.MonitorStateWaitSyncs)
		return true
	}
	if time.Since(t.waitSyncsSince) > t.waitSyncsTimeout() {
		t.log.Errorf("syncs %s still running after %s: the instance is not stopped", strings.Join(rids, ","), t.waitSyncsTimeout())
		t.waitSyncsSince = time.Time{}
		t.stopRefusedForSyncs = true
		t.transitionTo(instance.MonitorStateStopFailure)
		return true
	}
	return true
}

// resumeMonitorIfStopRefused turns the monitoring back on when the stop that
// failed was refused for the syncs running: nothing was stopped, so an
// orchestration that turned the monitoring off to stop leaves the instance
// running as it found it. A stop that failed half way leaves it off.
func (t *Manager) resumeMonitorIfStopRefused() {
	if !t.stopRefusedForSyncs {
		return
	}
	t.stopRefusedForSyncs = false
	if t.instStatus[t.localhost].Avail.Is(status.Up, status.Warn) {
		t.enableMonitor("stop refused while syncs run")
	}
}

// onWaitSyncsTimer has the orchestration look again at the syncs it waits
// for, their time being up, when no status change did.
func (t *Manager) onWaitSyncsTimer() {
	if t.state.State != instance.MonitorStateWaitSyncs {
		return
	}
	t.onChange()
}
