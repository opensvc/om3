package daemonapi

import (
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/opensvc/om3/v3/daemon/proc"

	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
	"github.com/opensvc/om3/v3/util/xsession"
)

// apiExec forks the command and returns the ids naming what it forked: the
// session, which a command reaching several objects or several nodes shares
// between all of them, and the exec, which is this one alone. A client is
// handed both so it can ask after the whole of what it submitted, or after
// the part that ran here.
func (a *DaemonAPI) apiExec(ctx echo.Context, p naming.Path, requesterSessionID uuid.UUID, args []string, log *plog.Logger) (uuid.UUID, uuid.UUID, error) {
	execname, err := os.Executable()
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("can't detect om execname: %w", err)
	}
	sessionID := xsession.NewSessionID(requesterSessionID)
	execID := xsession.NewExecID()
	cmd := command.New(
		command.WithName(execname),
		command.WithArgs(args),
		command.WithLogger(log),
		command.WithVarEnv(
			env.ActionOriginDaemonAPI.Var(),
			sessionID.Var(),
			execID.Var(),
			"OSVC_REQUEST_ID="+fmt.Sprint(ctx.Get("uuid")),
		),
	)
	// The node label is what a subscriber narrows on to hear only what this
	// node runs, and every other publisher of these messages sets it.
	labels := []pubsub.Label{labelOriginAPI, {"node", a.localhost}}
	if !p.IsZero() {
		labels = append(labels, pubsub.Label{"namespace", p.Namespace}, pubsub.Label{"path", p.String()})
	}
	log.Infof("-> exec %s", cmd)
	msg := msgbus.Exec{
		Command:   cmd.String(),
		Node:      a.localhost,
		Origin:    "api",
		SessionID: sessionID,
		ExecID:    execID,
	}
	a.Bus.Pub(&msg, labels...)
	startTime := time.Now()
	if err = cmd.Start(); err != nil {
		// The start of this exec was announced, so its end has to be too, or
		// it stays running in the exec store for as long as the store keeps
		// it. There is no exit status to report: it never ran.
		a.Bus.Pub(&msgbus.ExecFailed{
			Command:   cmd.String(),
			Duration:  time.Now().Sub(startTime),
			ErrS:      err.Error(),
			ExitCode:  -1,
			Node:      a.localhost,
			Origin:    "api",
			SessionID: sessionID,
			ExecID:    execID,
		}, labels...)
		log.Errorf("exec StartProcess: %s", err)
		return sessionID.UUID(), execID.UUID(), fmt.Errorf("instance action failed: %w", err)
	}
	pid := cmd.Cmd().Process.Pid
	proc.Register(proc.T{Pid: pid, ExecID: execID.String()})
	go func() {
		err := cmd.Wait()
		proc.Unregister(pid)
		log.Infof("<- exec %s", cmd)
		duration := time.Now().Sub(startTime)
		if err != nil {
			msg := msgbus.ExecFailed{
				Command:   cmd.String(),
				Duration:  duration,
				Node:      a.localhost,
				Origin:    "api",
				SessionID: sessionID,
				ExecID:    execID,
				ErrS:      err.Error(),
				ExitCode:  cmd.NormalizedExitCode(),
			}
			a.Bus.Pub(&msg, labels...)
		} else {
			msg := msgbus.ExecSuccess{
				Command:   cmd.String(),
				Duration:  duration,
				ExitCode:  cmd.NormalizedExitCode(),
				Node:      a.localhost,
				Origin:    "api",
				SessionID: sessionID,
				ExecID:    execID,
			}
			a.Bus.Pub(&msg, labels...)
		}
	}()
	return sessionID.UUID(), execID.UUID(), nil
}
