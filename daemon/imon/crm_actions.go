package imon

import (
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/core/env"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/daemon/runner"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/pubsub"
)

var (
	cmdPath string

	// testCRMAction can be used to define alternate testCRMAction for tests
	testCRMAction func(title string, cmdArgs ...string) error

	kindWithNotApplicableStatus = naming.NewKinds(naming.KindSvc, naming.KindVol)
)

func init() {
	var err error
	cmdPath, err = os.Executable()
	if err != nil {
		cmdPath = "/bin/false"
	}
}

// SetCmdPathForTest set the opensvc command path for tests
func SetCmdPathForTest(s string) {
	// TODO use another method to create dedicated side effects for tests
	cmdPath = s
}

func (t *Manager) orchestrateAfterAction(state, newState instance.MonitorState) {
	select {
	case <-t.ctx.Done():
		return
	default:
	}
	t.cmdC <- cmdOrchestrate{state: state, newState: newState}
}

func (t *Manager) queueBoot() error {
	return runner.Run(t.instConfig.Priority, func() error {
		return t.crmBoot()
	})
}

func (t *Manager) queueFreeze() error {
	return runner.Run(t.instConfig.Priority, func() error {
		return t.crmFreeze()
	})
}

func (t *Manager) runStatus() error {
	errC := make(chan error)
	t.queueStatus(errC)
	return <-errC
}

func (t *Manager) queueStatus(errC chan error) {
	if !kindWithNotApplicableStatus.Has(t.path.Kind) {
		// no need for crm status action, intead simulate status with post status event
		naStatus := instance.Status{
			Avail:       status.NotApplicable,
			Optional:    status.NotApplicable,
			Overall:     status.NotApplicable,
			Provisioned: provisioned.NotApplicable,
			UpdatedAt:   time.Now(),
		}
		t.publisher.Pub(&msgbus.InstanceStatusPost{Path: t.path, Node: t.localhost, Value: naStatus}, t.pubLabels...)
		return
	}
	if t.statusQueued.Load() {
		t.needStatus.Store(true)
		return
	} else {
		t.statusQueued.Store(true)
	}

	fn := func() error {
		t.statusQueued.Store(false)
		t.needStatus.Store(false)
		err := t.crmStatus()
		if err != nil && errC == nil {
			// if errC is not nil, let the caller decide if/how he wants to log the err
			t.log.Warnf("status evaluation command: %s", err)
		}
		return err
	}
	runner.Enqueue(t.instConfig.Priority, errC, fn)
}

func (t *Manager) queueResourceStartStandby(rids []string) error {
	return runner.Run(t.instConfig.Priority, func() error {
		return t.crmResourceStartStandby(rids)
	})
}

func (t *Manager) queueResourceStart(rids []string) error {
	return runner.Run(t.instConfig.Priority, func() error {
		return t.crmResourceStart(rids)
	})
}

func (t *Manager) queueUnfreeze() error {
	return runner.Run(t.instConfig.Priority, func() error {
		return t.crmUnfreeze()
	})
}

func (t *Manager) crmBoot() error {
	return t.crmAction("boot", t.path.String(), "instance", "boot")
}

func (t *Manager) crmDelete() error {
	t.publisher.Pub(&msgbus.InstanceConfigDeleting{
		Path: t.path,
		Node: t.localhost,
	}, t.pubLabels...)
	return t.crmAction("delete", t.path.String(), "instance", "delete")
}

func (t *Manager) crmFreeze() error {
	return t.crmAction("freeze", t.path.String(), "instance", "freeze")
}

func (t *Manager) crmProvisionNonLeader() error {
	return t.crmAction("provision non leader", t.path.String(), "instance", "provision")
}

func (t *Manager) crmProvisionLeader() error {
	return t.crmAction("provision leader", t.path.String(), "instance", "provision", "--leader", "--disable-rollback")
}

func (t *Manager) crmStartStandby() error {
	return t.crmAction("start", t.path.String(), "instance", "startstandby")
}

func (t *Manager) crmResourceStartStandby(rids []string) error {
	s := strings.Join(rids, ",")
	return t.crmAction("start", t.path.String(), "instance", "startstandby", "--rid", s)
}

func (t *Manager) crmResourceStart(rids []string) error {
	s := strings.Join(rids, ",")
	return t.crmAction("start", t.path.String(), "instance", "start", "--rid", s)
}

func (t *Manager) crmShutdown() error {
	return t.crmAction("shutdown", t.path.String(), "instance", "shutdown")
}

func (t *Manager) crmStart() error {
	return t.crmAction("start", t.path.String(), "instance", "start")
}

func (t *Manager) crmStatus() error {
	return t.crmAction("status", t.path.String(), "instance", "status", "-r")
}

func (t *Manager) crmStop() error {
	return t.crmAction("stop", t.path.String(), "instance", "stop")
}

func (t *Manager) crmUnfreeze() error {
	return t.crmAction("unfreeze", t.path.String(), "instance", "unfreeze")
}

func (t *Manager) crmUnprovisionNonLeader() error {
	return t.crmAction("unprovision non leader", t.path.String(), "instance", "unprovision")
}

func (t *Manager) crmUnprovisionLeader() error {
	return t.crmAction("unprovision leader", t.path.String(), "instance", "unprovision", "--leader")
}

func (t *Manager) crmAction(title string, cmdArgs ...string) error {
	if testCRMAction != nil {
		return testCRMAction(title, cmdArgs...)
	}
	return t.crmDefaultAction(title, cmdArgs...)
}

func (t *Manager) crmDefaultAction(title string, cmdArgs ...string) error {
	sid := uuid.New()
	cmd := command.New(
		command.WithName(cmdPath),
		command.WithArgs(cmdArgs),
		command.WithLogger(t.log),
		command.WithVarEnv(
			env.OriginSetenvArg(env.ActionOriginDaemonMonitor),
			env.ActionOrchestrationIDVar+"="+t.state.OrchestrationID.String(),
			"OSVC_SESSION_ID="+sid.String(),
		),
	)
	labels := append(t.pubLabels, pubsub.Label{"origin", "imon"})
	if title != "" {
		t.loggerWithState().Infof("-> exec %s", append([]string{cmdPath}, cmdArgs...))
	} else {
		t.loggerWithState().Debugf("-> exec %s", append([]string{cmdPath}, cmdArgs...))
	}
	t.publisher.Pub(&msgbus.Exec{Command: cmd.String(), Node: t.localhost, Origin: "imon", Title: title, SessionID: sid}, labels...)
	startTime := time.Now()
	if err := cmd.Run(); err != nil {
		duration := time.Now().Sub(startTime)
		t.publisher.Pub(&msgbus.ExecFailed{Command: cmd.String(), Duration: duration, ErrS: err.Error(), Node: t.localhost, Origin: "imon", Title: title, SessionID: sid}, labels...)
		t.loggerWithState().Errorf("<- exec %s: %s", append([]string{cmdPath}, cmdArgs...), err)
		return err
	}
	duration := time.Now().Sub(startTime)
	t.publisher.Pub(&msgbus.ExecSuccess{Command: cmd.String(), Duration: duration, Node: t.localhost, Origin: "imon", Title: title, SessionID: sid}, labels...)
	if title != "" {
		t.loggerWithState().Infof("<- exec %s", append([]string{cmdPath}, cmdArgs...))
	} else {
		t.loggerWithState().Debugf("<- exec %s", append([]string{cmdPath}, cmdArgs...))
	}
	return nil
}
