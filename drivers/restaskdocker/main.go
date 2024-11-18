package restaskpodman

// TODO
// * snooze
// * status.json rewrite after lock acquire

import (
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/drivers/rescontainerdocker"
	"github.com/opensvc/om3/drivers/rescontainerocibase"
	"github.com/opensvc/om3/drivers/restask"
	"github.com/opensvc/om3/drivers/restaskocibase"
)

type (
	// T is the driver structure.
	T struct {
		restaskocibase.T
	}
)

func New() resource.Driver {
	t := &T{
		T: restaskocibase.T{
			BaseTask: restask.BaseTask{
				T:            resource.T{},
				Check:        "",
				Confirmation: false,
				LogOutputs:   false,
				MaxParallel:  0,
				OnErrorCmd:   "",
				RetCodes:     "",
				RunTimeout:   nil,
				Schedule:     "",
				Snooze:       nil,
			},
		},
	}
	t.SetContainerGetter(t)
	return t
}

// GetContainerDetached returns a ContainerTasker where the base container has
// the Detach value set to false (task are never detached)
func (t *T) GetContainerDetached() restaskocibase.ContainerTasker {
	ct := &rescontainerdocker.T{
		BT: &rescontainerocibase.BT{
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
			Remove:                    t.Remove,
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
			StartTimeout:              t.Timeout,
		},
	}
	if err := ct.Configure(); err != nil {
		t.Log().Errorf("unable to configure docker task container")
	}
	ct.SetupExecutor()
	return ct
}
