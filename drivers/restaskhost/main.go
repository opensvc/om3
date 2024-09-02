package restaskhost

// TODO
// * snooze
// * status.json rewrite after lock acquire

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/env"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/drivers/resapp"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/confirmation"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/proc"
)

// T is the driver structure.
type T struct {
	resapp.T
	Check        string
	Confirmation bool
	LogOutputs   bool
	OnErrorCmd   string
	RunCmd       string
	RunTimeout   *time.Duration
	Schedule     string
	Snooze       *time.Duration
}

const (
	lockName = "run"
)

func New() resource.Driver {
	return &T{}
}

func (t T) IsRunning() bool {
	unlock, err := t.Lock(false, time.Second*0, lockName)
	if err != nil {
		return true
	}
	defer unlock()
	return false
}

func (t T) Run(ctx context.Context) error {
	disable := actioncontext.IsLockDisabled(ctx)
	timeout := actioncontext.LockTimeout(ctx)
	unlock, err := t.Lock(disable, timeout, lockName)
	if err != nil {
		return err
	}
	defer unlock()
	return t.lockedRun(ctx)
}

func (t T) loggerWithProc(p proc.T) *plog.Logger {
	return t.Log().Attr("cmd", p.CommandLine()).Attr("cmd_pid", p.PID())
}

func (t T) loggerWithCmd(cmd *command.T) *plog.Logger {
	return t.Log().Attr("cmd", cmd.String())
}

func (t T) lockedRun(ctx context.Context) (err error) {
	if !env.HasDaemonOrigin() {
		defer t.notifyRunDone()
	}
	var opts []funcopt.O
	if err := t.handleConfirmation(ctx); err != nil {
		return err
	}
	if opts, err = t.GetFuncOpts(t.RunCmd, "run"); err != nil {
		return err
	}
	if len(opts) == 0 {
		return nil
	}
	if err := t.ApplyPGChain(ctx); err != nil {
		return err
	}
	if t.LogOutputs {
		opts = append(opts,
			command.WithLogger(t.Log()),
			command.WithStdoutLogLevel(zerolog.InfoLevel),
			command.WithStderrLogLevel(zerolog.WarnLevel),
		)
	}
	opts = append(opts,
		command.WithTimeout(t.GetTimeout("run")),
		command.WithIgnoredExitCodes(),
	)
	cmd := command.New(opts...)
	t.loggerWithCmd(cmd).Infof("run %s", cmd)
	err = cmd.Run()
	if err := t.writeLastRun(cmd.ExitCode()); err != nil {
		return err
	}
	if err != nil {
		t.Log().Errorf("write last run: %s", err)
		if err := t.onError(); err != nil {
			t.Log().Warnf("on error: %s", err)
		}
	}
	if s, err := t.ExitCodeToStatus(cmd.ExitCode()); err != nil {
		return err
	} else if s != status.Up {
		return fmt.Errorf("command exited with code %d", cmd.ExitCode())
	}
	return nil
}

func (t T) onError() error {
	opts, err := t.GetFuncOpts(t.OnErrorCmd, "on_error")
	if err != nil {
		return err
	}
	if len(opts) == 0 {
		return nil
	}
	cmd := command.New(opts...)
	t.loggerWithCmd(cmd).Infof("on error run")
	return cmd.Run()
}

func (t *T) Kill(ctx context.Context) error {
	if t.StopCmd != "" {
		return t.CommonStop(ctx, t)
	}
	return t.stop(ctx)
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
	switch t.Check {
	case "last_run":
		return t.statusLastRun(ctx)
	default:
		return status.NotApplicable
	}
}

func (t T) writeLastRun(retcode int) error {
	p := t.lastRunFile()
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	fmt.Fprintf(f, "%d\n", retcode)
	return nil
}

func (t T) readLastRun() (int, error) {
	p := t.lastRunFile()
	if b, err := os.ReadFile(p); err != nil {
		return 0, err
	} else {
		return strconv.Atoi(strings.TrimSpace(string(b)))
	}
}

func (t T) lastRunFile() string {
	return filepath.Join(t.VarDir(), "last_run_retcode")
}

func (t *T) statusLastRun(ctx context.Context) status.T {
	if err := resource.StatusCheckRequires(ctx, t); err != nil {
		t.StatusLog().Info("requirements not met")
		return status.NotApplicable
	}
	if i, err := t.readLastRun(); err != nil {
		t.StatusLog().Info("never run")
		return status.NotApplicable
	} else {
		s, err := t.ExitCodeToStatus(i)
		if err != nil {
			t.StatusLog().Info("%s", err)
		}
		if s != status.Up {
			t.StatusLog().Info("last run failed (%d)", i)
		}
		return s
	}
}

func (t *T) running(ctx context.Context) bool {
	var s status.T
	if t.CheckCmd != "" {
		s = t.CommonStatus(ctx)
	} else {
		s = t.status()
	}
	return s == status.Up
}

// Label returns a formatted short description of the Resource
func (t T) Label() string {
	return ""
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
	switch procs.Len() {
	case 0:
		return status.Down
	case 1:
		return status.Up
	default:
		t.StatusLog().Warn("too many process (%d)", procs.Len())
		return status.Up
	}
}

func (t T) getRunning(cmdArgs []string) (proc.L, error) {
	procs, err := proc.All()
	if err != nil {
		return procs, err
	}
	procs = procs.FilterByEnv("OPENSVC_ID", t.ObjectID.String())
	procs = procs.FilterByEnv("OPENSVC_RID", t.RID())
	return procs, nil
}

func (t T) handleConfirmation(ctx context.Context) error {
	if !t.Confirmation {
		return nil
	}
	if actioncontext.IsConfirm(ctx) {
		t.Log().Infof("run confirmed by --confirm command line option")
		return nil
	}
	if actioncontext.IsCron(ctx) {
		// as set by the daemon scheduler subsystem
		return fmt.Errorf("run aborted (--cron)")
	}
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		return fmt.Errorf("run aborted (stdin is not a tty)")
	}
	description := fmt.Sprintf(`The resource %s requires a run confirmation.
Please make sure you fully understand its role and effects before confirming the run.
Enter "yes" if you really want to run.`, t.RID())
	s, err := confirmation.ReadLn(description, time.Second*30)
	if err != nil {
		return fmt.Errorf("read confirmation: %w", err)
	}
	if s == "yes" {
		t.Log().Infof("run confirmed interactively")
		return nil
	}
	return fmt.Errorf("run aborted")
}

// notifyRunDone is a noop here as for now the daemon api has no support for
// POST /run_done, and may not need one.
func (t T) notifyRunDone() error {
	return nil
}

func (t T) ScheduleOptions() resource.ScheduleOptions {
	return resource.ScheduleOptions{
		Action:              "run",
		Option:              "schedule",
		Base:                "",
		RequireConfirmation: t.Confirmation,
		RequireProvisioned:  true,
		RequireCollector:    false,
	}
}
