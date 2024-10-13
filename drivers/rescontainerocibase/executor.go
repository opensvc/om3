package rescontainerocibase

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/opensvc/om3/util/plog"
)

type (
	Executor struct {
		bin       string
		args      ExecutorArgser
		inspecter Inspecter
		inspected bool
		logger    Logger
	}
)

func NewExecutor(exe string, args ExecutorArgser, log Logger) *Executor {
	return &Executor{bin: exe, args: args, logger: log}
}

func (e *Executor) HasImage(ctx context.Context) (bool, error) {
	var cmd *exec.Cmd
	a := e.args.HasImageArgs()
	if ctx != nil {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
			cmd = exec.CommandContext(ctx, e.bin, a.AsStrings()...)
		}
	} else {
		cmd = exec.Command(e.bin, a.AsStrings()...)
	}
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}

func (e *Executor) Inspect() Inspecter {
	if !e.inspected {
		e.log().Warnf("inspect called before Inspect refreshed, use dedicated context")
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		defer cancel()
		i, _ := e.InspectRefresh(ctx)
		return i
	}
	return e.inspecter
}

func (e *Executor) InspectRefresh(ctx context.Context) (Inspecter, error) {
	var cmd *exec.Cmd
	a := e.args.InspectArgs()
	if ctx != nil {
		select {
		case <-ctx.Done():
			e.log().Debugf("inspect context done: %s", ctx.Err())
			return nil, ctx.Err()
		default:
			cmd = exec.CommandContext(ctx, e.bin, a.AsStrings()...)
		}
	} else {
		cmd = exec.Command(e.bin, a.AsStrings()...)
	}
	e.inspected = true
	e.log().Debugf("engine inspect: %s %s", e.bin, strings.Join(a.Obfuscate(), " "))
	if b, err := cmd.Output(); err != nil {
		e.inspecter = nil
		e.log().Debugf("inspect: %s", err)
		return nil, nil
	} else if i, err := e.args.InspectParser(b); err != nil {
		e.inspecter = nil
		e.log().Debugf("inspect parse: %s", err)
		return nil, err
	} else {
		e.inspecter = i
		e.log().Debugf("inspect success")
		return i, nil
	}
}

func (e *Executor) InspectRefreshed() bool {
	return e.inspected
}

func (e *Executor) Pull(ctx context.Context) error {
	return e.do(ctx, e.args.PullArgs())
}

func (e *Executor) Remove(ctx context.Context) error {
	return e.do(ctx, e.args.RemoveArgs())
}

func (e *Executor) Run(ctx context.Context) error {
	return e.do(ctx, e.args.RunArgs())
}

func (e *Executor) Start(ctx context.Context) error {
	return e.do(ctx, e.args.StartArgs())
}

func (e *Executor) Stop(ctx context.Context) error {
	return e.do(ctx, e.args.StopArgs())
}

func (e *Executor) WaitNotRunning(ctx context.Context) error {
	return e.args.WaitNotRunning(ctx)
}

func (e *Executor) WaitRemoved(ctx context.Context) error {
	return e.args.WaitRemoved(ctx)
}

func (e *Executor) do(ctx context.Context, a Args) error {
	var cmd *exec.Cmd
	if ctx != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			cmd = exec.CommandContext(ctx, e.bin, a.AsStrings()...)
		}
	} else {
		cmd = exec.Command(e.bin, a.AsStrings()...)
	}
	e.log().Infof("%s %s", e.bin, strings.Join(a.Obfuscate(), " "))
	if err := cmd.Run(); err != nil {
		e.log().Infof("%s %s: %s", e.bin, strings.Join(a.Obfuscate(), " "), err)
		return err
	}
	return nil
}

func (e *Executor) log() *plog.Logger {
	return e.logger.Log()
}
