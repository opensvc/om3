package rescontainerpodman

import (
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/drivers/rescontainerocibase"
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
	t.configure(t.executorArg())
	return nil
}

func (t *T) configure(ea *ExecutorArg) {
	executor := rescontainerocibase.NewExecutor("podman", ea, t)
	_ = t.WithExecuter(executor)
}

func (t *T) executorArg() *ExecutorArg {
	baseArgs := []string{
		"--cgroup-manager", "cgroupfs",
	}
	if t.CNIConfig != "" {
		baseArgs = append(baseArgs, "--cni-config-dir", t.CNIConfig)
	}

	return &ExecutorArg{
		ExecutorArg: &rescontainerocibase.ExecutorArg{
			BT: &t.BT,

			RunArgsDNSOptionOption: "--dns-opt",
		},

		exe: "podman",

		baseArgs: baseArgs,
	}
}
