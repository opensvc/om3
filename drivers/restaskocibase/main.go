package restaskocibase

// TODO
// * snooze
// * status.json rewrite after lock acquire

import (
	"context"
	"fmt"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/rescontainerocibase"
	"github.com/opensvc/om3/v3/drivers/restask"
	"github.com/opensvc/om3/v3/util/pg"
)

type (
	// T is the driver structure.
	T struct {
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
		ReadOnly        string         `json:"read_only"`
		RegistryCreds   string         `json:"registry_creds"`
		PullTimeout     *time.Duration `json:"pull_timeout"`
		Timeout         *time.Duration `json:"timeout"`

		containerDetachedGetter ContainerDetachedGetter
	}

	ContainerDetachedGetter interface {
		GetContainerDetached() ContainerTasker
	}

	ContainerTasker interface {
		Start(context.Context) error
		Stop(context.Context) error
		ContainerInspectRefresh(context.Context) (rescontainerocibase.Inspecter, error)
		Signal(context.Context, syscall.Signal) error
	}
)

func (t *T) SetContainerGetter(c ContainerDetachedGetter) {
	t.containerDetachedGetter = c
}

func (t *T) Run(ctx context.Context) error {
	return t.RunIf(ctx, t.lockedRun)
}

func (t *T) Stop(ctx context.Context) (err error) {
	container := t.containerDetachedGetter.GetContainerDetached()

	if container == nil {
		t.Log().Tracef("stop container skipped: container is absent")
		return nil
	}

	if err := container.Stop(ctx); err != nil {
		t.Log().Errorf("stop: %s", err)
		return err
	}

	return nil
}

func (t *T) lockedRun(ctx context.Context) (err error) {
	container := t.containerDetachedGetter.GetContainerDetached()

	if container == nil {
		return fmt.Errorf("unable to get task container")
	}

	startErr := container.Start(ctx)

	// TODO: handle rm = true, detach = true ?

	inspect, err := container.ContainerInspectRefresh(ctx)
	if err != nil {
		if startErr != nil {
			return fmt.Errorf("inspect error: %w after a start error: %w", err, startErr)
		}
		return err
	}

	if inspect == nil {
		err := fmt.Errorf("unable to inspect task container to retrieve its exit code")
		if startErr != nil {
			return fmt.Errorf("inspect error: %w after a start error: %w", err, startErr)
		}
		return err
	}
	exitCode := inspect.ExitCode()
	if err := t.WriteLastRun(exitCode); err != nil {
		t.Log().Errorf("write last run: %s", err)
		return err
	}
	if s, err := t.BaseTask.ExitCodeToStatus(exitCode); err != nil {
		return err
	} else if s != status.Up {
		return fmt.Errorf("command exited with code %d", exitCode)
	}
	return nil
}

func (t *T) Kill(ctx context.Context) error {
	container := t.containerDetachedGetter.GetContainerDetached()
	return container.Signal(ctx, syscall.SIGKILL)
}

func (t *T) running(ctx context.Context) bool {
	c := t.containerDetachedGetter.GetContainerDetached()
	inspect, err := c.ContainerInspectRefresh(ctx)
	if err != nil || inspect == nil {
		return false
	}
	return inspect.Running()
}

// Label returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	return ""
}
