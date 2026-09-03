package rescontainerpodman

import (
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/drivers/rescontainerocibase"
)

type (
	T struct {
		rescontainerocibase.BT
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

// executorArg returns the executor of the podman commands.
//
// Its base args are empty: this driver never asks podman to build a network.
// The netns keyword resolves to "host", to a private namespace, or to the one
// of another container, and the addresses are configured by the ip drivers,
// from outside. Podman therefore never reads a network configuration, and the
// "--cni-config-dir" this used to pass is both inert and, since podman 5
// dropped the cni backend, an unknown flag.
func (t *T) executorArg() *ExecutorArg {
	var baseArgs []string

	return &ExecutorArg{
		ExecutorArg: &rescontainerocibase.ExecutorArg{
			BT: &t.BT,

			RunArgsDNSOptionOption: "--dns-opt",
		},

		exe: "podman",

		baseArgs: baseArgs,
	}
}
