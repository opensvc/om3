package imon

import (
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/daemon/runner"
	"github.com/opensvc/om3/v3/util/file"
)

// orchestrateCapped applies the process group caps of the configuration to
// every running instance of the object.
//
// The caps are the pg_* keywords of the configuration, which the request
// wrote before the orchestration was queued: every node waits for that
// configuration to land, and then applies it where the instance runs. An
// instance not running has nothing to apply the caps to, and applies them
// when it starts.
//
// The nodes do not wait on each other: a cap is applied to the groups of the
// local instance alone. A failure is final on the instance it failed on, and
// is not retried.
func (t *Manager) orchestrateCapped() {
	switch t.state.State {
	case instance.MonitorStateIdle:
		t.cappedFromIdle()
	case instance.MonitorStateCapSuccess:
		t.cappedEnd("the instance runs with the caps configured", true)
	case instance.MonitorStateCapFailure:
		t.cappedEnd("applying the caps failed", false)
	}
}

func (t *Manager) cappedFromIdle() {
	if !t.hasCapConfig() {
		// Waiting, not failing: the configuration is on its way, and the
		// orchestration is re-evaluated as the node monitor changes.
		return
	}
	if !t.hasCapTarget() {
		t.log.Infof("cap: the instance is not running, it applies the caps when it starts")
		t.transitionTo(instance.MonitorStateCapSuccess)
		return
	}
	_ = runner.Run(t.instConfig.Priority, func() error {
		t.transitionTo(instance.MonitorStateCapProgress)
		next := instance.MonitorStateCapSuccess
		if err := t.crmPGUpdate(); err != nil {
			next = instance.MonitorStateCapFailure
		}
		go t.orchestrateAfterAction(instance.MonitorStateCapProgress, next)
		return nil
	})
}

// cappedEnd ends the orchestration on this instance. A success leaves the
// instance idle, since the caps it runs with are in the status, and a failure
// lingers, as the state of every failed action does.
func (t *Manager) cappedEnd(msg string, succeed bool) {
	if t.state.OrchestrationIsDone {
		return
	}
	if succeed {
		t.log.Infof("capped orchestration reached: %s", msg)
		t.doneAndIdle()
	} else {
		t.log.Infof("capped orchestration done: %s", msg)
		t.done()
	}
	t.updateIfChange()
}

// hasCapTarget says whether the local instance runs processes to apply the
// caps to: it is up, or runs its standby resources.
func (t *Manager) hasCapTarget() bool {
	return t.instStatus[t.localhost].Avail.Is(status.Up, status.Warn, status.StandbyUp, status.StandbyUpWithUp)
}

// hasCapConfig says whether this node holds the configuration the caps were
// written to.
//
// The caps are read from the local configuration, and a configuration write
// is acknowledged by the node that received it a moment before it reaches the
// others. Without this, a node told to apply the caps before the write lands
// applies the ones it was asked to replace, and says it is done. The file is
// read rather than the configuration the daemon caches, because the file is
// what the action is about to read.
func (t *Manager) hasCapConfig() bool {
	options, ok := t.state.GlobalExpectOptions.(instance.MonitorGlobalExpectOptionsCapped)
	if !ok || options.ConfigUpdatedAt.IsZero() {
		return true
	}
	mtime := file.ModTime(t.path.ConfigFile())
	if mtime.IsZero() || mtime.Before(options.ConfigUpdatedAt) {
		t.log.Infof("cap: wait for the configuration of %s to land here", options.ConfigUpdatedAt)
		return false
	}
	return true
}
