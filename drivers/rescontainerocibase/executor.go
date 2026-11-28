package rescontainerocibase

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/term"

	"github.com/opensvc/om3/v3/util/args"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	// Executor implements Executer interface to manage containers.
	Executor struct {
		// bin is the main container executor cli command
		bin string

		// args is the ExecutorArgser used by executor focusing on resource information
		args ExecutorArgser

		// inspected is set to true when container has been inspected at least
		// once.
		inspected bool

		// inspecter is the latest result of inspect refresh
		inspecter Inspecter

		// logger provides a resource logger for executor
		logger Logger

		mutex *sync.RWMutex
	}
)

var (
	// waitRemovedByInspectTimeout bounds the inspects judging a failed
	// wait for the removal of a container.
	waitRemovedByInspectTimeout = 10 * time.Second

	// waitRemovedByInspectInterval is the delay between these inspects.
	waitRemovedByInspectInterval = 500 * time.Millisecond
)

func NewExecutor(exe string, args ExecutorArgser, log Logger) *Executor {
	return &Executor{bin: exe, args: args, logger: log, mutex: &sync.RWMutex{}}
}

func (e *Executor) EncapCmd(ctx context.Context, args []string, env []string, stdin io.Reader) (*exec.Cmd, error) {
	var interactive bool
	if stdin != nil {
		interactive = true
	}
	args = e.args.ExecCmdArgs(args, env, interactive)
	cmd, err := e.command(ctx, args...)
	if err != nil {
		return nil, err
	}
	if stdin != nil {
		cmd.Stdin = stdin
	} else {
		cmd.Stdin = os.Stdin
	}
	return cmd, nil
}

func (e *Executor) EncapCp(ctx context.Context, src, dst string) error {
	args := e.args.CpCmdArgs(src, dst)
	return e.doExecRun(ctx, nil, args...)
}

// Enter runs a shell in the container, through the exec of the engine: the
// shell has the capabilities, the seccomp and security profiles, and the
// cgroup of the container, as the processes of the container, and no more.
//
// The client of an exec ending, as the hang up of the terminal of a console
// session ends it, leaves the shell it started running in the container: the
// shell and what it starts carry the environment variable marking the
// session, and are ended when the terminal hangs up.
func (e *Executor) Enter(ctx context.Context) error {
	var enterCmd string
	candidates := []string{"/bin/bash", "/bin/sh"}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	inspect, err := e.Inspect(ctx)
	cancel()
	if err != nil {
		return err
	}
	if inspect == nil {
		return fmt.Errorf("the container is not running")
	}
	pid := inspect.PID()
	if pid == 0 {
		// A container that exists and does not run has no process to
		// run a shell beside.
		return fmt.Errorf("the container is not running")
	}

	// The shell is looked for in the files of the container, not run to
	// see whether it exists: the exec of an engine tells a missing program
	// by an exit code it also gives other failures.
	for _, candidate := range candidates {
		if containerHasExecutable(pid, candidate) {
			enterCmd = candidate
			break
		}
	}
	if enterCmd == "" {
		return fmt.Errorf("can't enter: container needs at least one of following command: %s",
			strings.Join(candidates, ", "))
	}
	marker, err := newEnterMarker()
	if err != nil {
		return err
	}
	tty := term.IsTerminal(int(os.Stdin.Fd()))
	cmd, err := e.command(nil, e.args.EnterCmdArgs(enterCmd, marker, tty)...)
	if err != nil {
		return err
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	hangup := make(chan os.Signal, 1)
	signal.Notify(hangup, syscall.SIGHUP, syscall.SIGTERM)
	defer signal.Stop(hangup)

	if err := cmd.Start(); err != nil {
		return err
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	select {
	case err = <-exited:
	case <-hangup:
		endEnterSession(pid, marker, enterKillGrace)
		select {
		case err = <-exited:
		case <-time.After(enterKillGrace):
			_ = cmd.Process.Kill()
			err = <-exited
		}
	}
	if err != nil {
		switch cmd.ProcessState.ExitCode() {
		case 130:
			return nil
		}
		return err
	}
	return nil
}

// HasImage says whether the store of the engine holds the image of the
// container, and its id. An engine that does not answer is an error, not a
// missing image: see hasImage.
func (e *Executor) HasImage(ctx context.Context) (bool, string, error) {
	if ctx != nil {
		select {
		case <-ctx.Done():
			return false, "", ctx.Err()
		default:
		}
	}
	run := func(ctx context.Context, a ...string) ([]byte, error) {
		a = e.getArgs(a...)
		cmd, err := e.command(ctx, a...)
		if err != nil {
			return nil, err
		}
		e.log().Tracef("call %s %s", e.bin, a)
		b, err := cmd.Output()
		if err != nil {
			e.log().Tracef("call %s %s failed: %s", e.bin, a, err)
		}
		return b, err
	}
	return hasImage(ctx, run, e.args.HasImageArgs().Get())
}

// Inspect returns Inspecter from cache. On cache miss a new Inspecter is
// created from InspectRefresh(ctx).
func (e *Executor) Inspect(ctx context.Context) (Inspecter, error) {
	if i, ok := e.inspectFromCache(); ok {
		return i, nil
	}
	return e.InspectRefresh(ctx)
}

// InspectRefresh creates new Inspecter (from inspect command line). It updates
// e Inspecter cache that may be used Inspect(ctx).
func (e *Executor) InspectRefresh(ctx context.Context) (Inspecter, error) {
	a := e.getArgs(e.args.InspectArgs().Get()...)
	if ctx != nil {
		select {
		case <-ctx.Done():
			e.log().Errorf("inspect context done: %s", ctx.Err())
			return nil, ctx.Err()
		default:
		}
	}
	cmd, err := e.command(ctx, a...)
	if err != nil {
		return nil, err
	}
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.inspected = true
	e.log().Tracef("engine inspect: %s %s", e.bin, strings.Join(a, " "))
	if b, err := cmd.Output(); err != nil {
		e.inspecter = nil
		e.log().Tracef("inspect: %s", err)
		return nil, nil
	} else if i, err := e.args.InspectParser(b); err != nil {
		e.inspecter = nil
		e.log().Tracef("inspect parse: %s", err)
		return nil, err
	} else {
		e.inspecter = i
		e.log().Tracef("inspect success")
		return i, nil
	}
}

// Pull pulls the image of the container. Its output is logged, and what the
// engine says of a failure is kept for the error too: the error of a
// prefetch is read in the status of the resource, far from the logs.
func (e *Executor) Pull(ctx context.Context) error {
	return e.doExec(ctx, true, true, nil, e.args.PullArgs().Get()...)
}

func (e *Executor) Remove(ctx context.Context) error {
	if err := e.doExecRun(ctx, nil, e.args.RemoveArgs().Get()...); err != nil {
		if inspect, err := e.InspectRefresh(ctx); err == nil && inspect == nil {
			e.log().Tracef("remove: container removed")
			return nil
		}
		return err
	}
	return nil
}

func (e *Executor) Run(ctx context.Context) error {
	if a, err := e.args.RunArgsBase(ctx); err != nil {
		return fmt.Errorf("can't prepare base args for run command: %s", err)
	} else if environ, err := e.args.RunCmdEnv(); err != nil {
		return fmt.Errorf("can't prepare run command environ: %s", err)
	} else {
		if imageArgs, err := e.args.RunArgsImage(); err != nil {
			return fmt.Errorf("can't prepare image args for run command: %s", err)
		} else if commandArgs, err := e.args.RunArgsCommand(); err != nil {
			return fmt.Errorf("can't prepare command args for run command: %s", err)
		} else {
			a.Append(imageArgs.Get()...)
			a.Append(commandArgs.Get()...)
			return e.doExecRunLog(ctx, !e.quietRun(), environ, a.Get()...)
		}
	}
}

func (e *Executor) Start(ctx context.Context) error {
	a, err := e.args.StartArgs(ctx)
	if err != nil {
		return err
	}
	return e.doExecRunLog(ctx, !e.quietRun(), nil, a.Get()...)
}

func (e *Executor) Stop(ctx context.Context) error {
	if err := e.doExecRun(ctx, nil, e.args.StopArgs().Get()...); err != nil {
		if inspect, err := e.InspectRefresh(ctx); err == nil && inspect == nil {
			e.log().Tracef("stop: container removed")
			return nil
		}
		return err
	}
	return nil
}

// Pause freezes the processes of the container.
func (e *Executor) Pause(ctx context.Context) error {
	return e.doExecRun(ctx, nil, e.args.PauseArgs().Get()...)
}

// Unpause thaws the processes of the container.
func (e *Executor) Unpause(ctx context.Context) error {
	return e.doExecRun(ctx, nil, e.args.UnpauseArgs().Get()...)
}

// WaitNotRunning waits for the container to be not running.
//
// A wait the engine fails is judged by inspect: the container gone or not
// running is what the wait was for. An engine may not know the options of
// the wait, as podman before 4.3 has no --ignore and podman 1 no --condition.
func (e *Executor) WaitNotRunning(ctx context.Context) error {
	err := e.args.WaitNotRunning(ctx)
	if err == nil {
		return nil
	}
	inspect, inspectErr := e.InspectRefresh(ctx)
	switch {
	case inspectErr != nil, ctxDone(ctx):
		// An inspect the context killed says nothing of the container.
		return err
	case inspect == nil:
		e.log().Tracef("wait not running: container removed")
		return nil
	case !inspect.Running():
		e.log().Debugf("wait not running: %s: the container is not running", err)
		return nil
	default:
		return err
	}
}

// ctxDone tells if ctx, which may be nil, is done.
func ctxDone(ctx context.Context) bool {
	return ctx != nil && ctx.Err() != nil
}

// WaitRemoved waits for the container to be removed.
//
// A wait the engine fails, as WaitNotRunning, is judged by inspect, for a
// while: the engine may remove an auto-removed container a moment after its
// stop.
func (e *Executor) WaitRemoved(ctx context.Context) error {
	err := e.args.WaitRemoved(ctx)
	if err == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, waitRemovedByInspectTimeout)
	defer cancel()
	ticker := time.NewTicker(waitRemovedByInspectInterval)
	defer ticker.Stop()
	for {
		inspect, inspectErr := e.InspectRefresh(ctx)
		if inspectErr == nil && inspect == nil && !ctxDone(ctx) {
			// Not an inspect the context killed: the container is gone.
			e.log().Debugf("wait removed: %s: the container is removed", err)
			return nil
		}
		select {
		case <-ctx.Done():
			return err
		case <-ticker.C:
		}
	}
}

// ExecutorArgser implements ExecutorArgserGetter for external tests
func (e *Executor) ExecutorArgser() ExecutorArgser {
	return e.args
}

// doExecRun exec e.bin a where a may be prefixed by baseArgs when e.args
// implements ExecutorBaseArgser, its output logged.
// Depending on ctx value, exec.Command or exec.CommandContext is used.
func (e *Executor) doExecRun(ctx context.Context, environ map[string]string, a ...string) error {
	return e.doExecRunLog(ctx, true, environ, a...)
}

// doExecRunLog exec e.bin a where a may be prefixed by baseArgs when e.args
// implements ExecutorBaseArgser.
// Depending on ctx value, exec.Command or exec.CommandContext is used.
// When logOutput is true the stdout lines are logged at info level and the
// stderr ones at warn level. When false, the output is read elsewhere, as
// the one of a task container is from its logs after the run, and the last
// stderr line, what the engine says of a failure, is kept for the error:
// that line only, a container writing on stderr for as long as it runs.
func (e *Executor) doExecRunLog(ctx context.Context, logOutput bool, environ map[string]string, a ...string) error {
	return e.doExec(ctx, logOutput, !logOutput, environ, a...)
}

// doExec is doExecRunLog, with the last stderr line kept for the error when
// keepReason is true, whether the output is logged or not.
func (e *Executor) doExec(ctx context.Context, logOutput, keepReason bool, environ map[string]string, a ...string) error {
	cmdArgs := e.getArgs(a...)
	opts, err := e.commandOpts(ctx, environ, cmdArgs)
	if err != nil {
		return err
	}
	var last string
	if logOutput {
		opts = append(opts,
			command.WithLogger(e.log()),
			command.WithStdoutLogLevel(zerolog.InfoLevel),
			command.WithStderrLogLevel(zerolog.WarnLevel),
		)
	}
	if keepReason {
		opts = append(opts, command.WithOnStderrLine(func(s string) {
			if s = strings.TrimSpace(s); s != "" {
				last = s
			}
		}))
	}

	cmd := command.New(opts...)

	e.log().Infof("%s %s", e.bin, strings.Join(cmdArgs, " "))
	err = cmd.Run()
	if err != nil && last != "" {
		return fmt.Errorf("%w: %s", err, last)
	}
	return err
}

// commandOpts returns the options of a command executing e.bin with
// cmdArgs: the environ, the credential of a rootless engine, and ctx.
func (e *Executor) commandOpts(ctx context.Context, environ map[string]string, cmdArgs []string) ([]funcopt.O, error) {
	opts := []funcopt.O{
		command.WithName(e.bin),
		command.WithArgs(cmdArgs),
	}

	cred, err := e.credential()
	if err != nil {
		return nil, err
	}
	if len(environ) > 0 || cred != nil {
		envL := os.Environ()
		for k, v := range environ {
			e.log().Tracef("exec with env %s=xxx", k)
			envL = append(envL, fmt.Sprintf("%s=%s", k, v))
		}
		if cred != nil {
			envL = append(envL, cred.Env...)
		}
		opts = append(opts, command.WithEnv(envL))
	}
	if cred != nil {
		opts = append(opts,
			command.WithUser(fmt.Sprint(cred.UID)),
			command.WithGroup(fmt.Sprint(cred.GID)),
		)
		if cred.Home != "" {
			opts = append(opts, command.WithCWD(cred.Home))
		}
	}

	if ctx != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			opts = append(opts, command.WithContext(ctx))
		}
	}
	return opts, nil
}

// quietRun tells whether the output of the commands running the container,
// run and start, is read elsewhere than as it comes, as the one of a task
// container is.
func (e *Executor) quietRun() bool {
	i, ok := e.args.(interface{ QuietRunOutput() bool })
	return ok && i.QuietRunOutput()
}

// LogsSince calls onStdout and onStderr with the lines the container wrote
// on its stdout and on its stderr since since.
func (e *Executor) LogsSince(ctx context.Context, since time.Time, onStdout, onStderr func(string)) error {
	i, ok := e.args.(interface{ LogsSinceArgs(time.Time) *args.T })
	if !ok {
		return fmt.Errorf("executor does not support logs")
	}
	cmdArgs := e.getArgs(i.LogsSinceArgs(since).Get()...)
	opts, err := e.commandOpts(ctx, nil, cmdArgs)
	if err != nil {
		return err
	}
	opts = append(opts,
		command.WithOnStdoutLine(onStdout),
		command.WithOnStderrLine(onStderr),
	)
	return command.New(opts...).Run()
}

func (e *Executor) log() *plog.Logger {
	return e.logger.Log()
}

// getArgs returns a that may be prefixed by baseArgs when e.args implements
// ExecutorBaseArgser.
// NamesByLabel returns the names of the containers, running or not, carrying
// the label, a key=value pair.
func (e *Executor) NamesByLabel(ctx context.Context, label string) ([]string, error) {
	cmdArgs := e.getArgs("container", "ls", "--all", "--filter", "label="+label, "--format", "{{.Names}}")
	opts, err := e.commandOpts(ctx, nil, cmdArgs)
	if err != nil {
		return nil, err
	}
	opts = append(opts, command.WithBufferedStdout())
	cmd := command.New(opts...)
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	names := make([]string, 0)
	for _, s := range strings.Split(string(cmd.Stdout()), "\n") {
		if s = strings.TrimSpace(s); s != "" {
			names = append(names, s)
		}
	}
	return names, nil
}

func (e *Executor) getArgs(a ...string) []string {
	var cmdArgs []string
	if i, ok := e.args.(ExecutorBaseArgser); ok {
		cmdArgs = append(cmdArgs, i.ExecBaseArgs()...)
	}
	cmdArgs = append(cmdArgs, a...)
	return cmdArgs
}

func (e *Executor) inspectFromCache() (Inspecter, bool) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	if e.inspected {
		return e.inspecter, true
	}
	return nil, false
}

func (e *Executor) Logs(ctx context.Context, follow bool, lines int) (<-chan []byte, error) {
	if logArgser, ok := e.args.(ExecutorLogArgser); !ok {
		return nil, fmt.Errorf("executor does not support logs")
	} else {
		args := e.getArgs(logArgser.LogsArgs(follow, lines).Get()...)
		return e.doExecRunLogs(ctx, args...)
	}
}

func (e *Executor) doExecRunLogs(ctx context.Context, a ...string) (<-chan []byte, error) {
	cmdArgs := e.getArgs(a...)

	// Create a command
	cmd, err := e.command(ctx, cmdArgs...)
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	// Create a channel for log lines
	logChan := make(chan []byte)

	var wg sync.WaitGroup
	readStream := func(name string, stream io.Reader) {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			n, err := stream.Read(buf)
			if n > 0 {
				// Copy the data to a new slice to avoid race conditions
				data := make([]byte, n)
				copy(data, buf[:n])
				select {
				case logChan <- data:
				case <-ctx.Done():
					return
				}
			}
			if err != nil {
				if err != io.EOF {
					e.log().Warnf("error reading container logs %s: %s", name, err)
				}
				return
			}
		}
	}

	wg.Add(2)
	go readStream("stdout", stdout)
	go readStream("stderr", stderr)

	go func() {
		wg.Wait()
		// Wait for command to finish
		_ = cmd.Wait()
		close(logChan)
	}()

	return logChan, nil
}
