package rescontainerdockercli

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/drivers/rescontainerocibase"
)

type (
	T struct {
		*rescontainerocibase.BT
	}

	ExecutorArg struct {
		*rescontainerocibase.ExecutorArg
		exe              string
		inspectRefresher inspectRefresher
	}

	inspectRefresher interface {
		InspectRefresh(context.Context) (rescontainerocibase.Inspecter, error)
	}
)

func New() resource.Driver {
	bt := &rescontainerocibase.BT{}
	t := &T{BT: bt}
	executorArg := &ExecutorArg{
		ExecutorArg: &rescontainerocibase.ExecutorArg{
			BT:                     bt,
			RunArgsDNSOptionOption: "--dns-option",
		},
		exe: "docker",
	}
	executor := rescontainerocibase.NewExecutor("docker", executorArg, t)
	executorArg.inspectRefresher = executor
	_ = t.WithExecuter(executor)
	return t
}

func (ea *ExecutorArg) WaitNotRunning(ctx context.Context) error {
	var cmd *exec.Cmd
	a := rescontainerocibase.Args{
		{Option: "container"},
		{Option: "wait"},
		{Option: ea.BT.ContainerName()},
	}
	if ctx != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			cmd = exec.CommandContext(ctx, ea.exe, a.AsStrings()...)
		}
	} else {
		cmd = exec.Command(ea.exe, a.AsStrings()...)
	}
	ea.Log().Infof("%s %s", ea.exe, strings.Join(a.Obfuscate(), " "))
	if err := cmd.Run(); err != nil {
		ea.Log().Infof("%s %s: %s", ea.exe, strings.Join(a.Obfuscate(), " "), err)
		return err
	}
	return nil
}

func (ea *ExecutorArg) WaitRemoved(ctx context.Context) error {
	if ctx != nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
	}
	if removed, err := ea.isRemoved(ctx); err != nil {
		return err
	} else if removed {
		return nil
	}
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if removed, err := ea.isRemoved(ctx); err != nil {
				return err
			} else if removed {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (ea *ExecutorArg) isRemoved(ctx context.Context) (bool, error) {
	if inspect, err := ea.inspectRefresher.InspectRefresh(ctx); err == nil {
		ea.BT.Log().Debugf("is removed: %v", inspect == nil)
		return inspect == nil, nil
	} else {
		ea.BT.Log().Debugf("is removed: false")
		return false, err
	}
}
