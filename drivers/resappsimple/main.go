package resappsimple

import (
	"context"
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/core/actionrollback"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/resourceselector"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/drivers/resapp"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/proc"
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
	if err := t.ApplyPGChain(ctx); err != nil {
		return err
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

func (t *T) Stop(ctx context.Context) error {
	var err error
	if t.StopCmd != "" {
		err = t.CommonStop(ctx, t)
	} else {
		err = t.stop(ctx)
	}
	if err != nil {
		if resourceselector.FromContext(ctx, nil).IsZero() {
			// compat b2.1: ignore app resource stop error
			t.Log().Warnf("ignored stop failure: %s", err)
			return nil
		} else {
			return err
		}
	}
	return nil
}

func (t *T) stop(ctx context.Context) error {
	cmdArgs, err := t.BaseCmdArgs(t.StartCmd, "stop")
	if err != nil {
		return err
	}
	procs, err := t.getRunning(cmdArgs)
	if err != nil {
		return err
	}
	if procs.Len() == 0 {
		t.Log().Infof("already stopped")
		return nil
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
	return t.status()
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	return drvID.String()
}

func (t *T) status() status.T {
	cmdArgs, err := t.BaseCmdArgs(t.StartCmd, "start")
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
