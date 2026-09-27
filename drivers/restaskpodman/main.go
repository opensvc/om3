package restaskpodman

// TODO
// * snooze
// * status.json rewrite after lock acquire

import (
	"context"
	"time"

	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/rescontainerocibase"
	"github.com/opensvc/om3/v3/drivers/rescontainerpodman"
	"github.com/opensvc/om3/v3/drivers/restaskocibase"
)

type (
	// T is the driver structure.
	T struct {
		restaskocibase.T

		// RootlessUser, when set, is the unprivileged user podman runs the
		// task container as.
		RootlessUser string `json:"rootless_user"`

		// RootlessGroup overrides the primary group of RootlessUser.
		RootlessGroup string `json:"rootless_group"`
	}
)

var _ resource.IDMapper = (*T)(nil)

func New() resource.Driver {
	t := &T{}
	t.SetContainerGetter(t)
	return t
}

// GetContainerDetached returns a ContainerTasker where the base container has
// the Detach value set to false (task are never detached).
func (t *T) GetContainerDetached() restaskocibase.ContainerTasker {
	return t.container()
}

func (t *T) container() *rescontainerpodman.T {
	var startTimeout *time.Duration

	// TODO: verify followoing rule
	if t.RunTimeout != nil {
		startTimeout = t.RunTimeout
	} else if t.Timeout != nil {
		startTimeout = t.Timeout
	}

	ct := &rescontainerpodman.T{
		BT: rescontainerocibase.BT{
			T:                         t.BaseTask.T,
			Detach:                    false, // don't hide the detach value
			SCSIPersistentReservation: t.SCSIPersistentReservation,
			PG:                        t.PG,
			Path:                      t.Path,
			ObjectID:                  t.ObjectID,
			SCSIReserv:                t.SCSIReserv,
			PromoteRW:                 t.PromoteRW,
			NoPreemptAbort:            t.NoPreemptAbort,
			OsvcRootPath:              t.OsvcRootPath,
			GuestOS:                   t.GuestOS,
			Name:                      t.Name,
			Hostname:                  t.Hostname,
			Image:                     t.Image,
			ImagePullPolicy:           t.ImagePullPolicy,
			CWD:                       t.CWD,
			User:                      t.User,
			Command:                   t.Command,
			DNS:                       t.DNS,
			DNSExtra:                  t.DNSExtra,
			DNSSearch:                 t.DNSSearch,
			RunArgs:                   t.RunArgs,
			Entrypoint:                t.Entrypoint,
			Remove:                    true,
			Privileged:                t.Privileged,
			Init:                      t.Init,
			Interactive:               t.Interactive,
			TTY:                       t.TTY,
			VolumeMounts:              t.VolumeMounts,
			Env:                       t.Env,
			SecretsEnv:                t.SecretsEnv,
			ConfigsEnv:                t.ConfigsEnv,
			Devices:                   t.Devices,
			NetNS:                     t.NetNS,
			UserNS:                    t.UserNS,
			PIDNS:                     t.PIDNS,
			IPCNS:                     t.IPCNS,
			UTSNS:                     t.UTSNS,
			RegistryCreds:             t.RegistryCreds,
			PullTimeout:               t.PullTimeout,
			StartTimeout:              startTimeout,
			LogOutputs:                t.LogOutputs,
		},
		RootlessUser:  t.RootlessUser,
		RootlessGroup: t.RootlessGroup,
	}
	if err := ct.Configure(); err != nil {
		t.Log().Errorf("unable to configure podman task container")
	}
	return ct
}

// ApplyPG caps the task container where podman places it, which is under the
// subtree systemd delegates to the user of a rootless container: the
// container of the task applies the groups of the task, which it shares.
func (t *T) ApplyPG(ctx context.Context) error {
	return t.container().ApplyPG(ctx)
}

// ResetPG lifts the capping of the groups the task container is placed in.
func (t *T) ResetPG(ctx context.Context) error {
	return t.container().ResetPG(ctx)
}

// Status says what keeps the task from running rootless on this node, in
// the status of the task, beside the one of its last run.
func (t *T) Status(ctx context.Context) status.T {
	for _, err := range t.container().RootlessIssues() {
		t.StatusLog().Warn("%s", err)
	}
	return t.T.Status(ctx)
}

// HostUID implements resource.IDMapper: the host uid the uid id of the task
// container runs as.
func (t *T) HostUID(id uint32) (uint32, error) {
	return t.container().HostUID(id)
}

// HostGID implements resource.IDMapper: the host gid the gid id of the task
// container runs as.
func (t *T) HostGID(id uint32) (uint32, error) {
	return t.container().HostGID(id)
}
