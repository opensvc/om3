package restaskdocker

// TODO
// * snooze
// * status.json rewrite after lock acquire

import (
	"context"
	"fmt"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/drivers/rescontainerdocker"
	"github.com/opensvc/om3/drivers/restask"
	"github.com/opensvc/om3/util/pg"
)

// T is the driver structure.
type T struct {
	restask.BaseTask
	resource.SCSIPersistentReservation
	Detach          bool           `json:"detach"`
	PG              pg.Config      `json:"pg"`
	Path            naming.Path    `json:"path"`
	ObjectID        uuid.UUID      `json:"object_id"`
	SCSIReserv      bool           `json:"scsireserv"`
	PromoteRW       bool           `json:"promote_rw"`
	NoPreemptAbort  bool           `json:"no_preempt_abort"`
	OsvcRootPath    string         `json:"osvc_root_path"`
	GuestOS         string         `json:"guest_os"`
	Name            string         `json:"name"`
	Nodes           []string       `json:"nodes"`
	Hostname        string         `json:"hostname"`
	Image           string         `json:"image"`
	ImagePullPolicy string         `json:"image_pull_policy"`
	CWD             string         `json:"cwd"`
	User            string         `json:"user"`
	Command         []string       `json:"command"`
	DNS             []string       `json:"dns"`
	DNSSearch       []string       `json:"dns_search"`
	RunArgs         []string       `json:"run_args"`
	Entrypoint      []string       `json:"entrypoint"`
	Remove          bool           `json:"remove"`
	Privileged      bool           `json:"privileged"`
	Init            bool           `json:"init"`
	Interactive     bool           `json:"interactive"`
	TTY             bool           `json:"tty"`
	VolumeMounts    []string       `json:"volume_mounts"`
	Env             []string       `json:"environment"`
	SecretsEnv      []string       `json:"secrets_environment"`
	ConfigsEnv      []string       `json:"configs_environment"`
	Devices         []string       `json:"devices"`
	NetNS           string         `json:"netns"`
	UserNS          string         `json:"userns"`
	PIDNS           string         `json:"pidns"`
	IPCNS           string         `json:"ipcns"`
	UTSNS           string         `json:"utsns"`
	RegistryCreds   string         `json:"registry_creds"`
	PullTimeout     *time.Duration `json:"pull_timeout"`
	Timeout         *time.Duration `json:"timeout"`
}

func New() resource.Driver {
	return &T{}
}

func (t T) Container() *rescontainerdocker.T {
	return &rescontainerdocker.T{
		T:                         t.BaseTask.T,
		Detach:                    false,
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
	}
}

func (t T) Run(ctx context.Context) error {
	return t.RunIf(ctx, t.lockedRun)
}

func (t T) lockedRun(ctx context.Context) (err error) {
	// TODO: if t.LogOutputs {}
	container := t.Container()
	if err := container.Start(ctx); err != nil {
		t.Log().Errorf("%s", err)
		return err
	}
	inspect, err := container.Inspect(ctx)
	if err != nil {
		return err
	}
	if err := t.WriteLastRun(inspect.State.ExitCode); err != nil {
		t.Log().Errorf("write last run: %s", err)
		return err
	}
	if s, err := t.BaseTask.ExitCodeToStatus(inspect.State.ExitCode); err != nil {
		return err
	} else if s != status.Up {
		return fmt.Errorf("command exited with code %d", inspect.State.ExitCode)
	}
	return nil
}

func (t *T) Kill(ctx context.Context) error {
	return t.Container().Signal(syscall.SIGKILL)
}

func (t *T) running(ctx context.Context) bool {
	inspect, err := t.Container().Inspect(ctx)
	if err != nil {
		return false
	}
	return inspect.State.Running
}

// Label returns a formatted short description of the Resource
func (t T) Label() string {
	return ""
}
