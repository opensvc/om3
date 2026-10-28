package resappsimple

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/sys/unix"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/resapp"
	"github.com/opensvc/om3/v3/util/bootid"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/converters"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/proc"
)

// T is the driver structure.
type T struct {
	resapp.T
	Kill string `json:"kill"`
}

const (
	// killParent makes the process the start command launched the one the
	// stop terminates and the status looks for. The processes it starts
	// carrying the resource identity are left running.
	killParent = "parent"

	// killTree makes every process carrying the resource identity in its
	// environment the ones the stop terminates and the status looks for.
	killTree = "tree"
)

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
	if err := t.recordStarted(cmd.Cmd().Process.Pid); err != nil {
		t.Log().Warnf("record the started process: %s", err)
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
	procs, err := t.getTargets(cmdArgs)
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
	same := sameStartAs(procs)
	for _, p := range procs.Procs() {
		t.loggerWithProc(p).Infof("send termination signal to process %d", p.PID())
		_, _ = p.SignalIf(syscall.SIGTERM, same)
	}
	prev := procs
	for i := 0; i < 5; i++ {
		procs, err := t.getTargets(cmdArgs)
		if err != nil {
			return err
		}
		for _, p := range prev.Procs() {
			if !procs.HasPID(p.PID()) {
				t.loggerWithProc(p).Infof("process %d is now terminated", p.PID())
			}
		}
		if procs.Len() == 0 {
			t.logLeftRunning(cmdArgs)
			return nil
		}
		prev = procs
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("waited too long for process %s to disappear", procs)
}

// Signal implements the signal action, as the signal of an install line
// asks a reload of the application whose configuration file changed. It
// sends sig to the processes the stop terminates, as the kill keyword says:
// every process of the resource by default, which reaches the application
// under the shell running its start command, and the process the start
// command launched with kill = parent, for a server passing a signal on to
// its workers as it chooses.
func (t *T) Signal(ctx context.Context, sig syscall.Signal) error {
	targets, err := t.getTargets(nil)
	if err != nil {
		return err
	}
	if targets.Len() == 0 {
		t.Log().Infof("skip signal %s: not running", unix.SignalName(sig))
		return nil
	}
	var errs error
	same := sameStartAs(targets)
	for _, p := range targets.Procs() {
		if sent, err := p.SignalIf(sig, same); err != nil {
			errs = errors.Join(errs, fmt.Errorf("signal %s to process %d: %w", unix.SignalName(sig), p.PID(), err))
		} else if sent {
			t.loggerWithProc(p).Infof("sent signal %s to process %d", unix.SignalName(sig), p.PID())
		}
	}
	return errs
}

// sameStartAs returns the check a process of l is still the one found when
// it is held, just before a signal: it started when it did then. A start time
// does not change, whatever the process does of its environment, and a pid
// another process took since started later. A process whose start time can
// not be read right after it was found is gone, and is not signaled.
func sameStartAs(l proc.L) func(proc.T) bool {
	started := make(map[int]uint64, l.Len())
	for _, p := range l.Procs() {
		if st, err := p.StartTime(); err == nil {
			started[p.PID()] = st
		}
	}
	return func(p proc.T) bool {
		want, ok := started[p.PID()]
		if !ok {
			return false
		}
		st, err := p.StartTime()
		return err == nil && st == want
	}
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
	procs, err := t.getTargets(cmdArgs)
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

// getTargets returns the processes the stop terminates and the status looks
// for, as the kill keyword says.
func (t *T) getTargets(cmdArgs []string) (proc.L, error) {
	procs, err := t.getRunning(cmdArgs)
	if err != nil || t.Kill != killParent {
		return procs, err
	}
	return t.parentOf(procs), nil
}

// parentOf returns the process the start command launched, if it still
// runs. It may no longer carry the resource identity, as a launcher clearing
// its environment before it exec's: the record names it all the same.
//
// Without a record of it, as for a process an older agent started, they are
// the topmost processes carrying the resource identity: those whose parent
// does not carry it. A process the application detached from its parent
// then counts as one, but a status saying down while the application runs
// would start it again.
func (t *T) parentOf(procs proc.L) proc.L {
	l := proc.NewList()
	pid, startTime, bootID, err := t.readStarted()
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			t.Log().Debugf("read the started process: %s", err)
		}
		for _, p := range procs.Procs() {
			if ppid, err := p.PPID(); err == nil && procs.HasPID(ppid) {
				continue
			}
			l.Add(p)
		}
		return l
	}
	if bootID != bootid.Get() {
		// the start times of the processes of another boot are no
		// telling them apart from those of this one.
		return l
	}
	p := proc.New(pid)
	if st, err := p.StartTime(); err != nil || st != startTime {
		// gone, or the pid reused.
		return l
	}
	if zombie, err := p.IsZombie(); err != nil || zombie {
		return l
	}
	l.Add(p)
	return l
}

// logLeftRunning says how many processes carrying the resource identity the
// stop left running, as kill=parent asks.
func (t *T) logLeftRunning(cmdArgs []string) {
	if t.Kill != killParent {
		return
	}
	procs, err := t.getRunning(cmdArgs)
	if err != nil || procs.Len() == 0 {
		return
	}
	t.Log().Infof("leave %d processes the application started running, as kill=parent asks: %s", procs.Len(), procs)
}

// startedFile is where the start records the process its command launched,
// as "<pid> <start time> <boot id>", the start time and the boot id telling
// a reused pid apart.
//
// The record outlives the process: without it, the processes the
// application detached from a stopped parent would be taken for it.
func (t *T) startedFile() string {
	return filepath.Join(t.VarDir(), "started")
}

func (t *T) recordStarted(pid int) error {
	startTime, err := proc.New(pid).StartTime()
	if err != nil {
		return err
	}
	bootID := bootid.Get()
	if bootID == "" {
		return fmt.Errorf("no boot id")
	}
	p := t.startedFile()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(fmt.Sprintf("%d %d %s\n", pid, startTime, bootID)), 0o644)
}

func (t *T) readStarted() (int, uint64, string, error) {
	b, err := os.ReadFile(t.startedFile())
	if err != nil {
		return 0, 0, "", err
	}
	words := strings.Fields(string(b))
	if len(words) != 3 {
		return 0, 0, "", fmt.Errorf("%s: want \"<pid> <start time> <boot id>\", got %q", t.startedFile(), b)
	}
	pid, err := strconv.Atoi(words[0])
	if err != nil {
		return 0, 0, "", fmt.Errorf("%s: %w", t.startedFile(), err)
	}
	startTime, err := strconv.ParseUint(words[1], 10, 64)
	if err != nil {
		return 0, 0, "", fmt.Errorf("%s: %w", t.startedFile(), err)
	}
	return pid, startTime, words[2], nil
}
