package rescontainerpodman

import (
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/drivers/rescontainer"
	"github.com/opensvc/om3/v3/drivers/rescontainerocibase"
)

type (
	T struct {
		rescontainerocibase.BT

		// RootlessUser, when set, is the unprivileged user podman runs the
		// container as, in their own store and their own user namespace.
		RootlessUser string `json:"rootless_user"`

		// RootlessGroup overrides the primary group of RootlessUser.
		RootlessGroup string `json:"rootless_group"`
	}

	ExecutorArg struct {
		*rescontainerocibase.ExecutorArg
		exe      string
		baseArgs []string
		t        *T
	}
)

// engine is the engine of the driver, as resource.Pull names it.
const engine = "podman"

var (
	_ resource.PullDescriber = (*T)(nil)
	_ resource.Prefetcher    = (*T)(nil)
)

func New() resource.Driver {
	return &T{}
}

func (t *T) Configure() error {
	t.SetImageStore(engine, t.RootlessUser)
	t.configure(t.executorArg())
	return nil
}

// PullOf implements resource.PullDescriber: the image of a rootless
// container is pulled to the store of its user, and the one of the others to
// the store of root.
func (t *T) PullOf(src resource.HoldSource) (resource.Pull, bool) {
	store, _ := src.Get("rootless_user")
	return rescontainerocibase.PullOf(engine, store, src)
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

	ea := &ExecutorArg{
		ExecutorArg: &rescontainerocibase.ExecutorArg{
			BT: &t.BT,
		},

		exe: "podman",

		baseArgs: baseArgs,

		t: t,
	}
	if t.RootlessUser != "" {
		ea.ExecutorArg.WriteResolvConf = func(resolvConf rescontainer.ResolvConf) (string, error) {
			u, err := t.rootlessUser()
			if err != nil {
				return "", err
			}
			return writeResolvConf(u, t.resolvConfRel(), resolvConf)
		}
	}
	return ea
}

// Credential implements rescontainerocibase.ExecutorCredentialer: the podman
// commands of a rootless container run as its user.
func (ea *ExecutorArg) Credential() (*rescontainerocibase.Credential, error) {
	if ea.t == nil {
		return nil, nil
	}
	u, err := ea.t.rootlessUser()
	if err != nil || u == nil {
		return nil, err
	}
	return u.credential(), nil
}
