package resappsimple

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/resapp"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/converters"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/proc"
)

// T is the driver structure.
type T struct {
	resapp.T
}

func New() resource.Driver {
	return &T{}
}

func (t *T) loggerWithCmd(cmd *command.T) *plog.Logger {
	return t.Log().Attr("cmd", cmd.String())
}

func (t *T) loggerWithProc(p proc.T) *plog.Logger {
	return t.Log().Attr("cmd", p.CommandLine()).Attr("cmd_pid", p.PID())
}

// Start the Resource
func (t *T) Start(ctx context.Context) (err error) {
	var opts []funcopt.O
	if opts, err = t.GetFuncOpts(ctx, t.StartCmd, "start"); err != nil {
		return err
	}
	if len(opts) == 0 {
		return nil
	}
	appStatus := t.Status(ctx)
	if appStatus == status.Up {
		t.Log().Infof("already up")
		return nil
	}
	opts = append(opts,
		command.WithLogger(t.Log()),
		command.WithErrorExitCodeLogLevel(zerolog.WarnLevel),
	)
	cmd := command.New(opts...)

	cmd.Cmd().SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Create a new session to avoid kill on exit
	}

	t.loggerWithCmd(cmd).Infof("run: %s", cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error)
	go func() {
		done <- cmd.Cmd().Wait()
	}()
	select {
	case <-time.After(20 * time.Millisecond):
		// the process is still running
	case err := <-done:
		if exitError, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("the command exited immediately: %s", exitError.ProcessState)
		} else if err != nil {
			return err
		}
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.Stop(ctx)
	})
	return nil
}

// Stop runs the stop command, if any, then terminates the processes the
// start command left running, as v2 did: a stop command telling the
// application to stop, or cleaning after it, does not always end the
// processes it was started as, and the resource would stay up. A stop
// command set to false does nothing.
func (t *T) Stop(ctx context.Context) error {
	var err error
	switch {
	case t.StopCmd == "":
		err = t.stop(ctx, false)
	case isFalse(t.StopCmd):
		return nil
	default:
		err = errors.Join(t.CommonStop(ctx, t), t.stop(ctx, true))
	}
	if err != nil {
		if !actioncontext.HasResourceSelector(ctx) {
			// compat b2.1: ignore app resource stop error
			t.Log().Warnf("ignored stop failure: %s", err)
			return nil
		} else {
			return err
		}
	}
	return nil
}

// isFalse says a command keyword is set to false, which means no command.
func isFalse(s string) bool {
	v, err := converters.Bool.Convert(s)
	return err == nil && !v.(bool)
}

// stop terminates the processes of the start command. After a stop command,
// those found are the ones it left running.
func (t *T) stop(ctx context.Context, afterStopCmd bool) error {
	cmdArgs, err := t.BaseCmdArgs(ctx, t.StartCmd, "stop")
	if err != nil {
		return err
	}
	procs, err := t.getRunning(cmdArgs)
	if err != nil {
		return err
	}
	if procs.Len() == 0 {
		if !afterStopCmd {
			t.Log().Infof("already stopped")
		}
		return nil
	}
	if afterStopCmd {
		t.Log().Infof("the stop command left %d processes running", procs.Len())
	}
	for _, p := range procs.Procs() {
		t.loggerWithProc(p).Infof("send termination signal to process %d", p.PID())
		p.Signal(syscall.SIGTERM)
	}
	prev := procs
	for i := 0; i < 5; i++ {
		procs, err := t.getRunning(cmdArgs)
		if err != nil {
			return err
		}
		for _, p := range prev.Procs() {
			if !procs.HasPID(p.PID()) {
				t.loggerWithProc(p).Infof("process %d is now terminated", p.PID())
			}
		}
		if procs.Len() == 0 {
			return nil
		}
		prev = procs
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("waited too long for process %s to disappear", procs)
}

func (t *T) Status(ctx context.Context) status.T {
	if t.CheckCmd != "" {
		return t.CommonStatus(ctx)
	}
	return t.status(ctx)
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	return t.Desc
}

func (t *T) status(ctx context.Context) status.T {
	cmdArgs, err := t.BaseCmdArgs(ctx, t.StartCmd, "start")
	if err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	}
	procs, err := t.getRunning(cmdArgs)
	if err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	}
	if procs.Len() == 0 {
		return status.Down
	}
	return status.Up
}

func (t *T) getRunning(cmdArgs []string) (proc.L, error) {
	procs, err := proc.All()
	if err != nil {
		return procs, err
	}
	ids := []string{
		"OPENSVC_ID",
		"OPENSVC_SVC_ID", // compat
	}
	procs = procs.FilterByEnvList(ids, t.ObjectID.String())
	procs = procs.FilterByEnv("OPENSVC_RID", t.RID())
	return procs, nil
}
