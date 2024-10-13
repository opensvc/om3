package rescontainerocibase

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/opensvc/om3/util/plog"
)

type (
	Engine struct {
		exe       string
		helper    EngineHelper
		inspecter Inspecter
		inspected bool
	}
)

func NewEngine(exe string, args EngineHelper) Container {
	return &Engine{exe: exe, helper: args}
}

func (e *Engine) IsNotFound(err error) bool {
	return e.helper.IsNotFound(err)
}

func (e *Engine) Start(ctx context.Context) error {
	return e.do(ctx, "start", "starting", e.helper.StartArgs())
}

func (e *Engine) Stop(ctx context.Context) error {
	return e.do(ctx, "stop", "stopping", e.helper.StopArgs())
}

func (e *Engine) Run(ctx context.Context) error {
	return e.do(ctx, "run", "running", e.helper.RunArgs())
}

func (e *Engine) Pull(ctx context.Context) error {
	return e.do(ctx, "pull", "pulling", e.helper.PullArgs())
}

func (e *Engine) Remove(ctx context.Context) error {
	return e.do(ctx, "remove", "removing", e.helper.RemoveArgs())
}

func (e *Engine) Wait(ctx context.Context, condition ...WaitCondition) (bool, error) {
	var cmd *exec.Cmd
	a := e.helper.WaitArgs(condition...)
	if ctx != nil {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
			cmd = exec.CommandContext(ctx, e.exe, a.AsStrings()...)
		}
	} else {
		cmd = exec.Command(e.exe, a.AsStrings()...)
	}
	e.Log().Infof("%s: %s %s", "waiting", e.exe, strings.Join(a.Obfuscate(), " "))
	if b, err := cmd.Output(); err != nil {
		var hasNotFoundCondition bool
		for _, cond := range condition {
			if cond == WaitConditionNotRunning || cond == WaitConditionRemoved {
				hasNotFoundCondition = true
				break
			}
		}
		if hasNotFoundCondition && e.IsNotFound(err) {
			e.Log().Debugf("wait succeed on not found")
			return true, nil
		}
		e.Log().Errorf("wait command failed: %s (%s)", err, string(b))
		return false, err
	} else if len(b) > 0 {
		e.Log().Debugf("wait output: %s", string(b))
	}
	e.Log().Infof("wait ok")
	return true, nil
}

func (e *Engine) Create(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (e *Engine) HasImage(ctx context.Context) (bool, error) {
	var cmd *exec.Cmd
	a := e.helper.HasImageArgs()
	if ctx != nil {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
			cmd = exec.CommandContext(ctx, e.exe, a.AsStrings()...)
		}
	} else {
		cmd = exec.Command(e.exe, a.AsStrings()...)
	}
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}

func (e *Engine) do(ctx context.Context, action, actioning string, a Args) error {
	var cmd *exec.Cmd
	if ctx != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			cmd = exec.CommandContext(ctx, e.exe, a.AsStrings()...)
		}
	} else {
		cmd = exec.Command(e.exe, a.AsStrings()...)
	}
	e.Log().Infof("%s: %s %s", actioning, e.exe, strings.Join(a.Obfuscate(), " "))
	if err := cmd.Run(); err != nil {
		e.Log().Infof("%s failed %s %s: %s", action, e.exe, strings.Join(a.Obfuscate(), " "), err)
		return err
	}
	return nil
}

func (e *Engine) InspectRefreshed() bool {
	return e.inspected
}

func (e *Engine) InspectRefresh(ctx context.Context) (Inspecter, error) {
	var cmd *exec.Cmd
	a := e.helper.InspectArgs()
	if ctx != nil {
		select {
		case <-ctx.Done():
			e.Log().Debugf("inspect context done: %s", ctx.Err())
			return nil, ctx.Err()
		default:
			cmd = exec.CommandContext(ctx, e.exe, a.AsStrings()...)
		}
	} else {
		cmd = exec.Command(e.exe, a.AsStrings()...)
	}
	e.inspected = true
	e.Log().Debugf("♻️engine inspect: %s %s", e.exe, strings.Join(a.Obfuscate(), " "))
	if b, err := cmd.Output(); err != nil {
		e.inspecter = nil
		if e.helper.IsNotFound(err) {
			e.Log().Debugf("inspect: not found")
			return nil, nil
		} else {
			e.Log().Debugf("inspect: %s", err)
			return nil, err
		}
	} else if i, err := e.helper.InspectParser(b); err != nil {
		e.inspecter = nil
		e.Log().Debugf("inspect parse: %s", err)
		return nil, err
	} else {
		e.inspecter = i
		e.Log().Debugf("inspect success")
		return i, nil
	}
}

func (e *Engine) Inspect() Inspecter {
	if !e.inspected {
		e.Log().Warnf("😬inspect called before Inspect refreshed, use dedicated context")
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		defer cancel()
		i, _ := e.InspectRefresh(ctx)
		return i
	}
	return e.inspecter
}

func (e *Engine) Log() *plog.Logger {
	return e.helper.Log()
}
