package rescontainerdockercli

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
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
	t := &T{BT: &rescontainerocibase.BT{}}
	t.SetupExecutor()
	return t
}

func (t *T) SetupExecutor() {
	executorArg := &ExecutorArg{
		ExecutorArg: &rescontainerocibase.ExecutorArg{
			BT:                     t.BT,
			RunArgsDNSOptionOption: "--dns-option",
		},
		exe: "docker",
	}
	executor := rescontainerocibase.NewExecutor("docker", executorArg, t)
	_ = t.WithExecuter(executor)
}

// Status improve BT.Status with userns checks
func (t *T) Status(ctx context.Context) status.T {
	s := t.BT.Status(ctx)
	if s.Is(status.Up) {
		if t.BT.UserNS == "host" {
			expectedValue := "host"
			if inspect, err := t.ContainerInspect(ctx); err != nil {
				t.StatusLog().Warn("userns: can't verify value on nil inspect result")
			} else if inspectHostConfig := inspect.HostConfig(); inspectHostConfig == nil {
				t.StatusLog().Warn("userns: can't verify value on nil inspect HostConfig")
			} else if inspectHostConfig.UsernsMode != expectedValue {
				t.StatusLog().Warn("userns: UsernsMode is %s, should be %s", inspectHostConfig.UsernsMode, expectedValue)
			} else {
				t.Log().Debugf("valid userns: UsernsMode is %s", expectedValue)
			}
		}
	}
	return s
}

// RunArgsBase append extra args for docker
func (ea *ExecutorArg) RunArgsBase() (rescontainerocibase.Args, error) {
	a, err := ea.ExecutorArg.RunArgsBase()
	if err != nil {
		return nil, err
	}
	if len(ea.BT.UserNS) > 0 {
		if ea.BT.UserNS != "host" {
			return nil, fmt.Errorf("unexpected userns value '%s': the only valid value is 'hosts'", ea.BT.UserNS)
		}
		a = append(a, rescontainerocibase.Arg{
			Option:   "--userns",
			Value:    ea.BT.UserNS,
			HasValue: true,
		})
	}
	return a, nil
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
