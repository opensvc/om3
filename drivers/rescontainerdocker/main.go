package rescontainerdocker

import (
	"context"

	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/rescontainerocibase"
)

type (
	T struct {
		rescontainerocibase.BT
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
	return &T{}
}

func (t *T) Configure() error {
	t.configure(t.executorArg())
	return nil
}

func (t *T) configure(ea *ExecutorArg) {
	executor := rescontainerocibase.NewExecutor("docker", ea, t)
	ea.inspectRefresher = executor
	_ = t.WithExecuter(executor)
}

func (t *T) executorArg() *ExecutorArg {
	return &ExecutorArg{
		ExecutorArg: &rescontainerocibase.ExecutorArg{
			BT: &t.BT,

			RunArgsDNSOptionOption: "--dns-option",
		},
		exe: "docker",
	}
}

// Status improve BT.Status with userns checks
func (t *T) Status(ctx context.Context) status.T {
	s := t.BT.Status(ctx)
	if s.Is(status.Up) {
		if t.BT.UserNS == "host" {
			expectedValue := "host"
			if inspect, err := t.ContainerInspect(ctx); err != nil {
				t.StatusLog().Warn("userns: can't verify value on inspect failure: %s", err)
			} else if inspect == nil {
				t.StatusLog().Warn("userns: can't verify value on nil inspect result")
			} else if inspectHostConfig := inspect.HostConfig(); inspectHostConfig == nil {
				t.StatusLog().Warn("userns: can't verify value on nil inspect HostConfig")
			} else if inspectHostConfig.UsernsMode != expectedValue {
				t.StatusLog().Warn("userns: UsernsMode is %s, should be %s", inspectHostConfig.UsernsMode, expectedValue)
			} else {
				t.Log().Tracef("valid userns: UsernsMode is %s", expectedValue)
			}
		}
	}
	return s
}
