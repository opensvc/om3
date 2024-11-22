package rescontainerpodman

import (
	"context"
	"os/exec"
	"strings"

	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/drivers/rescontainerocibase"
	"github.com/opensvc/om3/util/args"
)

type (
	T struct {
		rescontainerocibase.BT

		CNIConfig string
	}

	ExecutorArg struct {
		*rescontainerocibase.ExecutorArg
		exe      string
		baseArgs []string
	}
)

func New() resource.Driver {
	return &T{}
}

func (t *T) Configure() error {
	baseArgs := []string{
		"--cgroup-manager", "cgroupfs",
	}
	if t.CNIConfig != "" {
		baseArgs = append(baseArgs, "--cni-config-dir", t.CNIConfig)
	}

	executorArg := &ExecutorArg{
		ExecutorArg: &rescontainerocibase.ExecutorArg{
			BT: &t.BT,

			RunArgsDNSOptionOption: "--dns-opt",
		},

		exe: "podman",

		baseArgs: baseArgs,
	}
	executor := rescontainerocibase.NewExecutor("podman", executorArg, t)
	_ = t.WithExecuter(executor)
	return nil
}

// RunArgsBase append extra args for podman
func (ea *ExecutorArg) RunArgsBase() (*args.T, error) {
	a := args.New()
	// TODO: "--cgroup-manager", "cgroupfs", "cni-config-dir", ..., for other Args ?
	if base, err := ea.ExecutorArg.RunArgsBase(); err != nil {
		return nil, err
	} else {
		a.Append(base.Get()...)
	}
	if len(ea.BT.UserNS) > 0 {
		isRawValue := func(s string) bool {
			return strings.HasPrefix(s, "auto") ||
				s == "host" ||
				strings.HasPrefix(s, "keep-id") ||
				strings.HasPrefix(s, "nomap") ||
				strings.HasPrefix(s, "ns:")
		}

		if isRawValue(ea.BT.UserNS) {
			a.Append("--userns", ea.BT.UserNS)
		} else if s, err := ea.BT.FormatNS(ea.BT.UserNS); err != nil {
			return nil, err
		} else {
			a.Append("--userns", s)
		}
	}
	return a, nil
}

func (ea *ExecutorArg) WaitRemoved(ctx context.Context) error {
	return ea.wait(ctx, "container", "wait", "--ignore", "--condition", "removing", ea.BT.ContainerName())
}

func (ea *ExecutorArg) WaitNotRunning(ctx context.Context) error {
	return ea.wait(ctx, "container", "wait", "--ignore", "--condition", "stopped", ea.BT.ContainerName())
}

func (ea *ExecutorArg) wait(ctx context.Context, a ...string) error {
	var cmd *exec.Cmd

	if ctx != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			cmd = exec.CommandContext(ctx, ea.exe, a...)
		}
	} else {
		cmd = exec.Command(ea.exe, a...)
	}
	ea.BT.Log().Infof("%s %s", ea.exe, strings.Join(a, " "))
	if err := cmd.Run(); err != nil {
		ea.BT.Log().Debugf("%s %s: %s", ea.exe, strings.Join(a, " "), err)
		return err
	}
	return nil
}

func (ea *ExecutorArg) ExecBaseArgs() []string {
	return ea.baseArgs
}
