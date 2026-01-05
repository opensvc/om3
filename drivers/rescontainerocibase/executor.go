package rescontainerocibase

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

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

func NewExecutor(exe string, args ExecutorArgser, log Logger) *Executor {
	return &Executor{bin: exe, args: args, logger: log, mutex: &sync.RWMutex{}}
}

func (e *Executor) EncapCmd(ctx context.Context, args []string, env []string, stdin io.Reader) (*exec.Cmd, error) {
	var interactive bool
	if stdin != nil {
		interactive = true
	}
	args = e.args.ExecCmdArgs(args, env, interactive)
	cmd := exec.CommandContext(ctx, e.bin, args...)
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

func (e *Executor) Enter(ctx context.Context) error {
	var enterCmd string
	candidates := []string{"/bin/bash", "/bin/sh"}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	inspect, err := e.Inspect(ctx)
	pid := inspect.PID()
	if err != nil {
		return err
	}
outerLoop:
	for _, candidate := range candidates {
		cmd := exec.CommandContext(ctx, "nsenter", "-t", fmt.Sprint(pid), "--all", "-e", "-w", candidate)
		_ = cmd.Run()

		switch cmd.ProcessState.ExitCode() {
		case 126, 127:
			continue
		default:
			enterCmd = candidate
			break outerLoop
		}
	}
	cancel()
	if enterCmd == "" {
		return fmt.Errorf("can't enter: container needs at least one of following command: %s",
			strings.Join(candidates, ", "))
	}
	cmd := exec.Command("nsenter", "-t", fmt.Sprint(pid), "--all", "-e", "-w", enterCmd)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		switch cmd.ProcessState.ExitCode() {
		case 130:
			return nil
		}
		return err
	}
	return nil
}

func (e *Executor) HasImage(ctx context.Context) (bool, string, error) {
	var cmd *exec.Cmd
	a := e.getArgs(e.args.HasImageArgs().Get()...)
	if ctx != nil {
		select {
		case <-ctx.Done():
			return false, "", ctx.Err()
		default:
			cmd = exec.CommandContext(ctx, e.bin, a...)
		}
	} else {
		cmd = exec.Command(e.bin, a...)
	}
	e.log().Tracef("call %s %s", e.bin, a)
	if b, err := cmd.Output(); err != nil {
		e.log().Tracef("call %s %s failed: %s", e.bin, a, err)
		return false, "", nil
	} else {
		imageID := strings.TrimSuffix(string(b), "\n")
		return true, imageID, nil
	}
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
	var cmd *exec.Cmd
	a := e.getArgs(e.args.InspectArgs().Get()...)
	if ctx != nil {
		select {
		case <-ctx.Done():
			e.log().Errorf("inspect context done: %s", ctx.Err())
			return nil, ctx.Err()
		default:
			cmd = exec.CommandContext(ctx, e.bin, a...)
		}
	} else {
		cmd = exec.Command(e.bin, a...)
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

func (e *Executor) Pull(ctx context.Context) error {
	return e.doExecRun(ctx, nil, e.args.PullArgs().Get()...)
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
			return e.doExecRun(ctx, environ, a.Get()...)
		}
	}
}

func (e *Executor) Start(ctx context.Context) error {
	a, err := e.args.StartArgs(ctx)
	if err != nil {
		return err
	}
	return e.doExecRun(ctx, nil, a.Get()...)
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

func (e *Executor) WaitNotRunning(ctx context.Context) error {
	if err := e.args.WaitNotRunning(ctx); err != nil {
		if inspect, err := e.InspectRefresh(ctx); err == nil && inspect == nil {
			e.log().Tracef("wait not running: container removed")
			return nil
		}
		return err
	}
	return nil
}

func (e *Executor) WaitRemoved(ctx context.Context) error {
	if err := e.args.WaitRemoved(ctx); err != nil {
		if inspect, err := e.InspectRefresh(ctx); err == nil && inspect == nil {
			e.log().Tracef("wait removed: container removed")
			return nil
		}
		return err
	}
	return nil
}

// ExecutorArgser implements ExecutorArgserGetter for external tests
func (e *Executor) ExecutorArgser() ExecutorArgser {
	return e.args
}

// doExecRun exec e.bin a where a may be prefixed by baseArgs when e.args
// implements ExecutorBaseArgser.
// Depending on ctx value, exec.Command or exec.CommandContext is used.
func (e *Executor) doExecRun(ctx context.Context, environ map[string]string, a ...string) error {
	return e.doExecRunLog(ctx, false, environ, a...)
}

// doExecRunLog exec e.bin a where a may be prefixed by baseArgs when e.args
// implements ExecutorBaseArgser.
// Depending on ctx value, exec.Command or exec.CommandContext is used.
// When logOutput is true it adds command options: WithLogger,
// WithStdoutLogLevel and WithStderrLogLevel
func (e *Executor) doExecRunLog(ctx context.Context, logOutput bool, environ map[string]string, a ...string) error {
	cmdArgs := e.getArgs(a...)
	opts := []funcopt.O{
		command.WithName(e.bin),
		command.WithArgs(cmdArgs),
	}
	if true {
		opts = append(opts,
			command.WithLogger(e.log()),
			command.WithStdoutLogLevel(zerolog.InfoLevel),
			command.WithStderrLogLevel(zerolog.WarnLevel),
		)
	}

	if len(environ) > 0 {
		envL := os.Environ()
		for k, v := range environ {
			e.log().Tracef("exec with env %s=xxx", k)
			envL = append(envL, fmt.Sprintf("%s=%s", k, v))
		}
		opts = append(opts, command.WithEnv(envL))
	}

	if ctx != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			opts = append(opts, command.WithContext(ctx))
		}
	}

	cmd := command.New(opts...)

	e.log().Infof("%s %s", e.bin, strings.Join(cmdArgs, " "))
	return cmd.Run()
}

func (e *Executor) log() *plog.Logger {
	return e.logger.Log()
}

// getArgs returns a that may be prefixed by baseArgs when e.args implements
// ExecutorBaseArgser.
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
