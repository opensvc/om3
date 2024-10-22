package rescontainerpodman

import (
	"context"
	"os/exec"
	"strings"

	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/drivers/rescontainerocibase"
)

type (
	T struct {
		*rescontainerocibase.BT
	}

	ExecutorArg struct {
		*rescontainerocibase.ExecutorArg
		exe string
	}
)

func New() resource.Driver {
	bt := &rescontainerocibase.BT{}
	t := &T{BT: bt}
	executorArg := &ExecutorArg{
		ExecutorArg: &rescontainerocibase.ExecutorArg{
			BT:                     bt,
			RunArgsDNSOptionOption: "--dns-opt",
		},
		exe: "podman",
	}
	executor := rescontainerocibase.NewExecutor("podman", executorArg, t)
	_ = t.WithExecuter(executor)
	return t
}

func (ea *ExecutorArg) WaitRemoved(ctx context.Context) error {
	a := rescontainerocibase.Args{
		{Option: "container"},
		{Option: "wait"},
		{Option: "--ignore"},
		{Option: "--condition", Value: "removing", HasValue: true},
		{Option: ea.BT.ContainerName()},
	}
	return ea.wait(ctx, a)
}

func (ea *ExecutorArg) WaitNotRunning(ctx context.Context) error {
	a := rescontainerocibase.Args{
		{Option: "container"},
		{Option: "wait"},
		{Option: "--ignore"},
		{Option: "--condition", Value: "stopped", HasValue: true},
		{Option: ea.BT.ContainerName()},
	}
	return ea.wait(ctx, a)
}

func (ea *ExecutorArg) wait(ctx context.Context, a rescontainerocibase.Args) error {
	var cmd *exec.Cmd
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
	ea.BT.Log().Infof("%s %s", ea.exe, strings.Join(a.Obfuscate(), " "))
	if err := cmd.Run(); err != nil {
		ea.BT.Log().Debugf("%s %s: %s", ea.exe, strings.Join(a.Obfuscate(), " "), err)
		return err
	}
	return nil
}
