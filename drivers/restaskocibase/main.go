package restaskocibase

// TODO
// * snooze
// * status.json rewrite after lock acquire

import (
	"context"
	"errors"
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
	"github.com/opensvc/om3/v3/util/signalrelay"
	"github.com/opensvc/om3/v3/util/xsession"
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
		DNSExtra        []string       `json:"dns_extra"`
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
		Sysctl          []string       `json:"sysctl"`
		StopTimeout     *time.Duration `json:"stop_timeout"`
		RegistryCreds   string         `json:"registry_creds"`
		PullTimeout     *time.Duration `json:"pull_timeout"`
		Timeout         *time.Duration `json:"timeout"`

		containerDetachedGetter ContainerDetachedGetter

		// runName is the name of the container of the run in progress,
		// when it has one of its own, see RunContainerName.
		runName string
	}

	ContainerDetachedGetter interface {
		GetContainerDetached() ContainerTasker
	}

	pgApplier interface {
		ApplyPG(context.Context) error
	}

	ContainerTasker interface {
		Start(context.Context) error
		Stop(context.Context) error
		RemoveContainer(context.Context) error
		ContainerInspectRefresh(context.Context) (rescontainerocibase.Inspecter, error)
		Signal(context.Context, syscall.Signal) error
		LabeledContainerNames(context.Context) ([]string, error)
	}
)

// RunContainerName returns the name of the task container: the name keyword,
// or the name of the resource container when empty. A task running several
// runs at once, max_parallel > 1, has a container per run, named after the
// exec id of the run: <name>.<exec id>.
func (t *T) RunContainerName() string {
	if t.runName != "" {
		return t.runName
	}
	return t.Name
}

// parallelRuns says the runs of the task have a container each.
func (t *T) parallelRuns() bool {
	return t.MaxParallel > 1
}

func (t *T) SetContainerGetter(c ContainerDetachedGetter) {
	t.containerDetachedGetter = c
}

func (t *T) Run(ctx context.Context) error {
	return t.RunIf(ctx, t.lockedRun)
}

// Stop stops the containers of the runs, then forgets the last run, as
// the base task does.
func (t *T) Stop(ctx context.Context) error {
	if err := t.stopContainers(ctx); err != nil {
		return err
	}
	return t.BaseTask.Stop(ctx)
}

func (t *T) stopContainers(ctx context.Context) error {
	if t.parallelRuns() {
		return t.stopRuns(ctx)
	}
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

// stopRuns stops the containers of the runs of a task running several at
// once, found by the label of the resource.
func (t *T) stopRuns(ctx context.Context) error {
	base := t.containerDetachedGetter.GetContainerDetached()
	if base == nil {
		return nil
	}
	names, err := base.LabeledContainerNames(ctx)
	if err != nil {
		return fmt.Errorf("list the run containers: %w", err)
	}
	var errs error
	for _, name := range names {
		t.runName = name
		if c := t.containerDetachedGetter.GetContainerDetached(); c != nil {
			if err := c.Stop(ctx); err != nil {
				errs = errors.Join(errs, fmt.Errorf("stop %s: %w", name, err))
			}
		}
	}
	t.runName = ""
	return errs
}

func (t *T) lockedRun(ctx context.Context) (err error) {
	if t.parallelRuns() {
		// Each run has a container of its own: kept, they would pile
		// up, one per run.
		if !t.Remove {
			return fmt.Errorf("max_parallel %d needs rm = true: each run has a container of its own, which a run would leave behind", t.MaxParallel)
		}
		base := rescontainerocibase.ContainerNameOf(t.Name, t.Path, t.ResourceID.String())
		t.runName = base + "." + xsession.ExecID().String()
		defer func() { t.runName = "" }()
	}
	if err := t.RunInstall(ctx); err != nil {
		return err
	}
	container := t.containerDetachedGetter.GetContainerDetached()

	if container == nil {
		return fmt.Errorf("unable to get task container")
	}

	// A container running is another run's, or one a run left behind:
	// starting it would start nothing, and counting it a run, its exit
	// code read while it runs, would count a run that did nothing.
	if inspect, err := container.ContainerInspectRefresh(ctx); err == nil && inspect != nil && inspect.Running() {
		return fmt.Errorf("the task is already running, in container %s", rescontainerocibase.ContainerNameOf(t.RunContainerName(), t.Path, t.ResourceID.String()))
	}
	if t.parallelRuns() {
		// The container of the run goes with it, whatever its end.
		defer func() {
			if err := container.RemoveContainer(ctx); err != nil {
				t.Log().Warnf("remove the run container %s: %s", t.RunContainerName(), err)
			}
		}()
	}

	// A run is not a start, so the action applies no group before it, and
	// the engine placed the container in a group made for it, uncapped: the
	// pg keywords of a task capped nothing. The driver applies them, and
	// through the driver rather than this base, which a podman task
	// overrides to place a rootless container's groups where podman puts
	// it.
	if i, ok := t.containerDetachedGetter.(pgApplier); ok {
		if err := i.ApplyPG(ctx); err != nil {
			return fmt.Errorf("apply pg: %w", err)
		}
	}

	// A signal the run receives, as the kill of its exec by the daemon
	// sends, stops the container as a stop does: the signal of the stop,
	// then a kill once its timeout expired, as a container whose first
	// process ignores the signal would run on. The run then ends as the
	// container did.
	if c, unsubscribe := signalrelay.Subscribe(ctx); c != nil {
		done := make(chan struct{})
		defer close(done)
		defer unsubscribe()
		go func() {
			select {
			case sig := <-c:
				t.Log().Infof("stop the container on signal %s", sig)
				if err := container.Stop(ctx); err != nil {
					t.Log().Warnf("stop the container on signal %s: %s", sig, err)
				}
			case <-done:
			}
		}()
	}

	startedAt := time.Now()
	startErr := container.Start(ctx)

	// TODO: handle detach = true ?

	inspect, err := container.ContainerInspectRefresh(ctx)
	if err != nil {
		if startErr != nil {
			return fmt.Errorf("inspect error: %w after a start error: %w", err, startErr)
		}
		return err
	}

	if inspect == nil {
		if sig, ok := signalrelay.Received(ctx).(syscall.Signal); ok {
			// The stop on the signal removed the container: the
			// run ended as the signal made it, 128 + the signal, as a
			// host task ended by it reports.
			if err := t.WriteLastRun(128 + int(sig)); err != nil {
				t.Log().Errorf("write last run: %s", err)
			}
			return fmt.Errorf("the container was stopped on signal %s", sig)
		}
		err := fmt.Errorf("unable to inspect task container to retrieve its exit code")
		if startErr != nil {
			return fmt.Errorf("inspect error: %w after a start error: %w", err, startErr)
		}
		return err
	}
	exitCode := inspect.ExitCode()
	if t.LogOutputs {
		t.logRunOutput(ctx, container, startedAt)
	}
	if err := t.WriteLastRun(exitCode); err != nil {
		t.Log().Errorf("write last run: %s", err)
		return err
	}
	if s, err := t.BaseTask.ExitCodeToStatus(exitCode); err != nil {
		return err
	} else if s != status.Up {
		return fmt.Errorf("command exited with code %d", exitCode)
	}
	if t.Remove && !t.parallelRuns() {
		if err := container.RemoveContainer(ctx); err != nil {
			return fmt.Errorf("remove container: %w", err)
		}
	}
	return nil
}

// logRunOutput logs what the container wrote during the run started at
// startedAt, read from its logs, as a host task logs its command output: the
// stdout lines at info level, the stderr ones at warn level. The run output
// is read from the logs rather than as it comes, so a run of a kept
// container, started rather than run, is logged as the run creating it is.
func (t *T) logRunOutput(ctx context.Context, container ContainerTasker, startedAt time.Time) {
	i, ok := container.(interface {
		LogsSince(context.Context, time.Time, func(string), func(string)) error
	})
	if !ok {
		return
	}
	err := i.LogsSince(ctx, startedAt,
		func(s string) {
			t.Log().Attr("out", s).Infof("stdout: %s", s)
		},
		func(s string) {
			t.Log().Attr("err", s).Warnf("stderr: %s", s)
		},
	)
	if err != nil {
		t.Log().Warnf("read the run output from the container logs: %s", err)
	}
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
