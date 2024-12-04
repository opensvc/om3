package restaskpodman

// TODO
// * snooze
// * status.json rewrite after lock acquire

import (
	"time"

	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/drivers/rescontainerocibase"
	"github.com/opensvc/om3/drivers/rescontainerpodman"
	"github.com/opensvc/om3/drivers/restaskocibase"
)

type (
	// T is the driver structure.
	T struct {
		restaskocibase.T

		CNIConfig string
	}
)

func New() resource.Driver {
	t := &T{}
	t.SetContainerGetter(t)
	return t
}

// GetContainerDetached returns a ContainerTasker where the base container has
// the Detach value set to false (task are never detached).
func (t *T) GetContainerDetached() restaskocibase.ContainerTasker {
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
		},
		CNIConfig: t.CNIConfig,
	}
	if err := ct.Configure(); err != nil {
		t.Log().Errorf("unable to configure podman task container")
	}
	return ct
}
