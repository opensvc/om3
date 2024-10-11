package rescontainerocibase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/core/actionrollback"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/resourceid"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/util/pg"
)

type (
	BT struct {
		resource.T
		resource.SCSIPersistentReservation
		ObjectDomain    string         `json:"object_domain"`
		PG              pg.Config      `json:"pg"`
		Path            naming.Path    `json:"path"`
		ObjectID        uuid.UUID      `json:"object_id"`
		SCSIReserv      bool           `json:"scsireserv"`
		PromoteRW       bool           `json:"promote_rw"`
		NoPreemptAbort  bool           `json:"no_preempt_abort"`
		OsvcRootPath    string         `json:"osvc_root_path"`
		GuestOS         string         `json:"guest_os"`
		Name            string         `json:"name"`
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
		Detach          bool           `json:"detach"`
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
		StartTimeout    *time.Duration `json:"start_timeout"`
		StopTimeout     *time.Duration `json:"stop_timeout"`

		CProvider Container
		Inspect   Inspecter
	}

	Arg struct {
		Short     string
		Long      string
		Default   string
		Obfuscate bool
		Multi     bool
	}

	Argser interface {
		Args() []Arg
	}

	ImagePullOptions struct {
		Name string
	}

	CreateOptions struct {
		Name  string
		Image string
	}

	containerNamer interface {
		ContainerName() string
	}

	Container interface {
		Create(ctx context.Context, options CreateOptions) error
		NewContainer(ctx context.Context, id string) (cs ContainerStarter, err error)
		Running(ctx context.Context, name string) (bool, error)
		Remove(ctx context.Context, name string) error
		Start(ctx context.Context, opts ...string) error
		Stop(ctx context.Context, name string) error
		Inspect(ctx context.Context, name string) (is Inspecter, err error)
		Wait(ctx context.Context, name string, opts ...WaitCondition) (int, error)
		Pull(ctx context.Context, opts ...string) error
		HasImage(ctx context.Context, id string) (exists bool, err error)
		PullOptions(bt *BT) ([]string, error)
		StartOptions(bt *BT) ([]string, error)
	}

	ContainerStarter interface {
		Start(ctx context.Context) error
		Wait(ctx context.Context, opts ...WaitCondition) (int, error)
	}

	ExitStatus interface {
		ExitCode() (int, error)
	}

	WaitCondition string

	Inspecter interface {
		HostConfigAutoRemove() bool
		ID() string
		Running() bool
		SandboxKey() string
		PID() int
	}
)

const (
	imagePullPolicyAlways = "always"
	imagePullPolicyOnce   = "once"
)

// ContainerName formats a docker container name
func (t *BT) ContainerName() string {
	if t.Name != "" {
		return t.Name
	}
	var s string
	switch t.Path.Namespace {
	case "root", "":
		s = ""
	default:
		s = t.Path.Namespace + ".."
	}
	s = s + t.Path.Name + "." + strings.ReplaceAll(t.ResourceID.String(), "#", ".")
	return s
}

func (t *BT) CreateOptions() (CreateOptions, error) {
	return CreateOptions{Name: t.Name}, nil
}

func (t *BT) StartOptions() []string {
	return nil
}

func (t *BT) IsAlwaysImagePullPolicy() bool {
	return t.ImagePullPolicy == imagePullPolicyAlways
}

func (t *BT) NeedPreStartRemove() bool {
	return t.Remove || !t.Detach
}

var (
	WaitConditionNotRunning = WaitCondition("not-running")
	WaitConditionRemoved    = WaitCondition("removed")

	ErrNotFound = errors.New("not found")
)

func (t *BT) Start(ctx context.Context) error {
	name := t.ContainerName()

	err := t.InspectRefresh(ctx)
	if err == nil {
		// has container
		if t.Inspect.Running() {
			t.Log().Infof("container is already running: %s", name)
			return nil
		} else {
			t.Log().Debugf("container is not running: %s", name)
			if t.NeedPreStartRemove() {
				t.Log().Infof("remove leftover container %s", name)
				if err := t.CProvider.Remove(ctx, name); err != nil {
					return err
				}
				if t.IsAlwaysImagePullPolicy() {
					if err := t.Pull(ctx); err != nil {
						return err
					}
				}
				return t.createAndStart(ctx)
			} else {
				id := t.Inspect.ID()
				t.Log().Infof("reuse container: %s (%s)", name, id)
				return t.findAndStart(ctx, id)
			}
		}
	} else if errors.Is(err, ErrNotFound) {
		t.Log().Debugf("container will be created: %s", name)
		if t.IsAlwaysImagePullPolicy() {
			t.Log().Debugf("pull image policy: always")
			if err := t.Pull(ctx); err != nil {
				return err
			}
		} else if ok, err := t.CProvider.HasImage(ctx, t.Image); err != nil {
			t.Log().Errorf("can't detect image for container %s: %s", name, err)
			return err
		} else if !ok {
			if err := t.Pull(ctx); err != nil {
				return err
			}
		}
		return t.createAndStart(ctx)
	} else {
		t.Log().Errorf("container inspect error for %s: %s", name, err)
		return err
	}
}

func (t *BT) Pull(ctx context.Context) error {
	if opt, err := t.CProvider.PullOptions(t); err != nil {
		t.Log().Errorf("can't detect pull options: %s", err)
		return err
	} else {
		t.Log().Infof("call: %s", strings.Join(opt, " "))
		if err := t.CProvider.Pull(ctx, opt...); err != nil {
			t.Log().Errorf("image pull failed: %s", err)
			return err
		}
		return nil
	}
}

func (t *BT) findAndStart(ctx context.Context, id string) error {
	if cs, err := t.CProvider.NewContainer(ctx, id); err != nil {
		return err
	} else {
		return t.start(ctx, cs)
	}
}

func (t *BT) Stop(ctx context.Context) error {
	name := t.ContainerName()
	err := t.InspectRefresh(ctx)
	if errors.Is(err, ErrNotFound) {
		return nil
	}

	if !t.Inspect.Running() {
		t.Log().Infof("container %s is already stopped", name)
	} else {
		if t.StopTimeout != nil && *t.StopTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, *t.StopTimeout)
			defer cancel()
			t.Log().Infof("stopping container %s with id %s (timeout %s)", name, t.Inspect.ID(), t.StopTimeout)
		} else {
			t.Log().Infof("stopping container %s with id %s", name, t.Inspect.ID())
		}
		err = t.CProvider.Stop(ctx, name)
		switch {
		case errors.Is(err, ErrNotFound):
			t.Log().Infof("stopped while requesting container %s stop", name)
		case err != nil:
			return err
		}
		t.Log().Debugf("stopped container %s: %s", name, err)
	}

	if t.Remove {
		if !t.Inspect.HostConfigAutoRemove() {
			t.Log().Infof("remove container %s", name)
			return t.CProvider.Remove(ctx, name)
		}
		t.Log().Debugf("wait removed condition")
		xc, err := t.CProvider.Wait(ctx, name, WaitConditionRemoved)
		switch {
		case errors.Is(err, ErrNotFound):
			t.Log().Infof("container %s not found while waiting removed", name)
		case err != nil:
			return err
		default:
			t.Log().Warnf("wait removed condition ended with exit code %d", xc)
		}
	}
	return nil
}

func (t *BT) FormatNS(s string) (string, error) {
	switch s {
	case "", "none", "host":
		return s, nil
	}
	rid, err := resourceid.Parse(s)
	if err != nil {
		return "", fmt.Errorf("invalid value %s (must be none, host or container#<n>)", s)
	}
	r := t.GetObjectDriver().ResourceByID(rid.String())
	if r == nil {
		return "", fmt.Errorf("resource %s not found", s)
	}
	if i, ok := r.(containerNamer); ok {
		name := i.ContainerName()
		return "container:" + name, nil
	}
	return "", fmt.Errorf("resource %s has no ns", s)
}

func (t *BT) Label() string {
	return t.Image
}

func (t *BT) Labels() map[string]string {
	data := make(map[string]string)
	data["com.opensvc.id"] = t.containerLabelID()
	data["com.opensvc.path"] = t.Path.String()
	data["com.opensvc.namespace"] = t.Path.Namespace
	data["com.opensvc.kind"] = t.Path.Kind.String()
	data["com.opensvc.name"] = t.Path.Name
	data["com.opensvc.rid"] = t.ResourceID.String()
	return data
}

func (t *BT) Status(ctx context.Context) status.T {
	if !t.Detach {
		return status.NotApplicable
	}

	err := t.InspectRefresh(ctx)
	if err == nil {
		// has container
		if t.Inspect.Running() {
			return status.Up
		} else {
			return status.Down
		}
	} else if errors.Is(err, ErrNotFound) {
		return status.Down
	} else {
		return status.Warn
	}
}

func (t *BT) InspectRefresh(ctx context.Context) error {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}
	inspect, err := t.CProvider.Inspect(ctx, t.ContainerName())
	if err != nil {
		return err
	}
	t.Inspect = inspect
	return nil
}

// NetNSPath implements the resource.NetNSPather optional interface.
// Used by ip.netns and ip.route to configure network stuff in the container.
func (t *BT) NetNSPath() (string, error) {
	if t.Inspect == nil {
		if err := t.InspectRefresh(nil); err != nil {
			return "", err
		}
	}
	return t.Inspect.SandboxKey(), nil
}

func (t *BT) PID() int {
	if t.Inspect == nil {
		if err := t.InspectRefresh(nil); err != nil {
			return 0
		}
	}
	return t.Inspect.PID()
}

func (t *BT) LinkNames() []string {
	return []string{t.RID()}
}

func (t *BT) Provision(_ context.Context) error {
	return nil
}

func (t *BT) Provisioned() (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

func (t *BT) Unprovision(_ context.Context) error {
	return nil
}

func (t *BT) containerLabelID() string {
	return fmt.Sprintf("%s.%s", t.ObjectID, t.ResourceID.String())
}

func (t *BT) createAndStart(ctx context.Context) error {
	if createOptions, err := t.CreateOptions(); err != nil {
		t.Log().Errorf("can't detect create options: %s", err)
		return err
	} else if err := t.CProvider.Create(ctx, createOptions); err != nil {
		t.Log().Errorf("Create with options %s: %s", createOptions, err)
		return err
	}
	startOptions, err := t.CProvider.StartOptions(t)
	if err != nil {
		t.Log().Errorf("can't detect start options: %s", err)
		return err
	}
	t.Log().Infof("call: %s", strings.Join(startOptions, " "))
	return t.CProvider.Start(ctx, startOptions...)
}

func (t *BT) start(ctx context.Context, cs ContainerStarter) error {
	errs := make(chan error, 1)
	go func() {
		if t.StartTimeout != nil {
			t.Log().Infof("start container (timeout %s)", t.StartTimeout)
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, *t.StartTimeout)
			defer cancel()
		} else {
			t.Log().Infof("start container (no timeout)")
		}
		if err := cs.Start(ctx); err != nil {
			errs <- err
			return
		}
		if t.Detach {
			errs <- nil
			return
		}
		if i, err := cs.Wait(ctx, WaitConditionNotRunning); err != nil {
			errs <- nil
			return
		} else {
			t.Log().Infof("foreground container exited with code %d)", i)
			errs <- nil
			return
		}
	}()

	var timerC <-chan time.Time
	if t.StartTimeout != nil {
		timerC = time.After(*t.StartTimeout)
	} else {
		timerC = make(<-chan time.Time)
	}
	select {
	case err := <-errs:
		if err == nil {
			actionrollback.Register(ctx, func() error {
				return t.Stop(ctx)
			})
		}
		return err
	case <-timerC:
		return fmt.Errorf("timeout")
	}
}
