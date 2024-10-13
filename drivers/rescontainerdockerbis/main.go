package rescontainerdockerbis

import (
	"errors"
	"os/exec"

	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/drivers/rescontainerocibase"
)

type (
	T struct {
		*rescontainerocibase.BT
	}

	C struct {
		*rescontainerocibase.EngineHelp
	}
)

func (c *C) WaitArgs(condition ...rescontainerocibase.WaitCondition) rescontainerocibase.Args {
	args := rescontainerocibase.Args{
		{Option: "container"},
		{Option: "wait"},
	}
	var ignoreNotRunning bool
	for _, cond := range condition {
		switch {
		case cond == rescontainerocibase.WaitConditionNotRunning:
			ignoreNotRunning = true
			arg := rescontainerocibase.Arg{Option: "--condition", Value: "stopped", HasValue: true}
			args = append(args, arg)
		case cond == rescontainerocibase.WaitConditionRemoved:
			ignoreNotRunning = true
			arg := rescontainerocibase.Arg{Option: "--condition", Value: "removing", HasValue: true}
			args = append(args, arg)
		default:
			arg := rescontainerocibase.Arg{Option: "--condition", Value: string(cond), HasValue: true}
			args = append(args, arg)
		}
	}
	if ignoreNotRunning {
		arg := rescontainerocibase.Arg{Option: "--ignore"}
		args = append(args, arg)
	}
	args = append(args, rescontainerocibase.Arg{Option: c.BT.ContainerName(), HasValue: true})
	return args
}

func New() resource.Driver {
	bt := &rescontainerocibase.BT{}
	t := &T{BT: bt}
	c := &C{
		EngineHelp: &rescontainerocibase.EngineHelp{BT: bt},
	}
	engine := rescontainerocibase.NewEngine("docker", c)
	t.BT = t.BT.WithEngine(engine)
	return t
}

func (c *C) IsNotFound(err error) bool {
	if errors.Is(err, rescontainerocibase.ErrNotFound) {
		return true
	}
	switch e := err.(type) {
	case *exec.ExitError:
		if e.ExitCode() == 125 {
			return true
		}
	}
	return false
}
