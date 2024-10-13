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

		c Container
	}

	containerNamer interface {
		ContainerName() string
	}

	Container interface {
		Start(context.Context) error
		Stop(context.Context) error
		Run(context.Context) error
		Remove(context.Context) error
		Create(ctx context.Context) error

		Pull(context.Context) error
		HasImage(context.Context) (bool, error)

		Inspect() Inspecter
		InspectRefresh(context.Context) (Inspecter, error)
		InspectRefreshed() bool

		IsNotFound(error) bool

		Wait(context.Context, ...WaitCondition) (bool, error)
	}

	ExitStatus interface {
		ExitCode() (int, error)
	}

	WaitCondition string

	Inspecter interface {
		Defined() bool
		ID() string
		HostConfigAutoRemove() bool
		PID() int
		Running() bool
		SandboxKey() string
	}
)

const (
	imagePullPolicyAlways = "always"
	imagePullPolicyOnce   = "once"
)

var (
	WaitConditionNotRunning = WaitCondition("not-running")
	WaitConditionRemoved    = WaitCondition("removed")

	ErrNotFound = errors.New("not found")
)

func (t *BT) WithEngine(c Container) *BT {
	t.c = c
	return t
}

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

func (t *BT) IsAlwaysImagePullPolicy() bool {
	return t.ImagePullPolicy == imagePullPolicyAlways
}

func (t *BT) NeedPreStartRemove() bool {
	return t.Remove || !t.Detach
}

func (t *BT) logMainAction(s string, err error) error {
	if err != nil {
		err = fmt.Errorf("%s: %s", s, err)
		t.Log().Errorf("%s", err)
		return err
	}
	return nil
}

func (t *BT) Start(ctx context.Context) error {
	name := t.ContainerName()
	//log := t.Log().WithPrefix(fmt.Sprintf("%s%s: ", t.Log().Prefix(), "start"))
	log := t.Log()

	logError := func(err error) error {
		return t.logMainAction("start", err)
	}

	inspect := t.c.Inspect()
	if inspect == nil || !inspect.Defined() {
		return logError(t.pullAndRun(ctx))
	}
	if inspect.Running() {
		log.Infof("container start %s: already started", name)
		return nil
	} else {
		// it is defined
		log.Debugf("container start %s: defined, but not started", name)
		if t.NeedPreStartRemove() {
			log.Infof("container start %s: remove leftover container", name)
			if err := t.c.Remove(ctx); err != nil {
				return logError(err)
			}
			return logError(t.pullAndRun(ctx))
		} else {
			return logError(t.findAndStart(ctx))
		}
	}
}

func (t *BT) Stop(ctx context.Context) error {
	name := t.ContainerName()
	log := t.Log()
	//log := t.Log().WithPrefix(fmt.Sprintf("%s%s: ", t.Log().Prefix(), "stop"))

	logError := func(err error) error {
		return t.logMainAction(fmt.Sprintf("container stop %s:", t.RID()), err)
	}

	inspect := t.c.Inspect()
	if inspect == nil || !inspect.Running() {
		log.Infof("already stopped")
		return nil
	}

	if t.StopTimeout != nil && *t.StopTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *t.StopTimeout)
		defer cancel()
		log.Debugf("stopping with timeout %s", *t.StopTimeout)
	} else {
		log.Debugf("stopping")
	}
	defer func() {
		_, _ = t.c.InspectRefresh(ctx)
	}()
	if err := t.c.Stop(ctx); err != nil {
		if t.c.IsNotFound(err) {
			log.Infof("container doesn't exist")
			return nil
		}
		return logError(err)
	}
	log.Debugf("container stopped")

	if t.Remove {
		if !inspect.HostConfigAutoRemove() {
			t.Log().Debugf("remove container %s", name)
			if err := t.c.Remove(ctx); err != nil {
				return logError(fmt.Errorf("can't remove container %s", name))
			}
		}
		t.Log().Debugf("wait removed condition")
		time.Sleep(200 * time.Millisecond)
		removed, err := t.c.Wait(ctx, WaitConditionRemoved)
		if err != nil {
			if t.c.IsNotFound(err) {
				t.Log().Debugf("removed")
				return nil
			} else {
				t.Log().Warnf("wait removed: %s", err)
				return err
			}
		}
		if removed {
			t.Log().Debugf("removed")
			return nil
		} else {
			t.Log().Warnf("wait removed failed")
			return fmt.Errorf("wait removed failed")
		}
	} else {
		t.Log().Debugf("wait not running condition")
		notRunning, err := t.c.Wait(ctx, WaitConditionNotRunning)
		if err != nil {
			if t.c.IsNotFound(err) {
				t.Log().Infof("wait running on not found")
				return nil
			} else {
				t.Log().Warnf("wait running: %s", err)
				return err
			}
		}
		if notRunning {
			t.Log().Debugf("wait running: not anymore running")
		} else {
			t.Log().Warnf("wait running: still running")
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
	var inspect Inspecter
	var err error
	t.Log().Debugf("Status.enter")
	defer t.Log().Debugf("Status.return")
	if !t.c.InspectRefreshed() {
		inspect, err = t.c.InspectRefresh(ctx)
		if err != nil {
			t.Log().Debugf("status warn on inspect refresh error: %s", err)
			return status.Warn
		}
	} else {
		inspect = t.c.Inspect()
	}
	if !t.Detach {
		t.Log().Debugf("status n/a on not dettach")
		return status.NotApplicable
	}
	if inspect == nil {
		t.Log().Debugf("status down on inspect nil")
		return status.Down
	} else if !inspect.Defined() {
		t.Log().Debugf("status down on inspect undefined")
		return status.Down
	} else if inspect.Running() {
		t.Log().Debugf("status up on inspect running")
		return status.Up
	} else {
		t.Log().Debugf("status down on inspect not running")
		return status.Down
	}
}

// NetNSPath implements the resource.NetNSPather optional interface.
// Used by ip.netns and ip.route to configure network stuff in the container.
func (t *BT) NetNSPath() (string, error) {
	if i := t.c.Inspect(); i == nil {
		return "", nil
	} else {
		return i.SandboxKey(), nil
	}
}

func (t *BT) PID() int {
	if i := t.c.Inspect(); i == nil {
		return 0
	} else {
		return i.PID()
	}
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

func (t *BT) pullAndRun(ctx context.Context) error {
	if t.IsAlwaysImagePullPolicy() {
		t.Log().Debugf("container start: with image policy: always")
		if err := t.pull(ctx); err != nil {
			return err
		}
	} else if hasImage, err := t.c.HasImage(ctx); err != nil {
		return fmt.Errorf("unable to detect if image %s exists localy: %s", t.Image, err)
	} else if !hasImage {
		if err := t.pull(ctx); err != nil {
			return err
		}
	}
	defer func() {
		_, _ = t.c.InspectRefresh(ctx)
	}()
	return t.c.Run(ctx)
}

func (t *BT) pull(ctx context.Context) error {
	if err := t.c.Pull(ctx); err != nil {
		return fmt.Errorf("can't pull image %s: %s", t.Image, err)
	}
	return nil
}

func (t *BT) findAndStart(ctx context.Context) error {
	name := t.ContainerName()
	i := t.c.Inspect()
	id := i.ID()
	errs := make(chan error, 1)
	go func() {
		if t.StartTimeout != nil && *t.StartTimeout > 0 {
			t.Log().Infof("container start %s (%s) with timeout %s", name, id, t.StartTimeout)
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, *t.StartTimeout)
			defer cancel()
		} else {
			t.Log().Infof("container start %s (%s) without timeout", name, id)
		}

		inspectRefresh := func() {
			_, err := t.c.InspectRefresh(context.Background())
			if err != nil {
				t.Log().Warnf("findAndStart InspectRefresh: %s", err)
			}
		}

		if err := t.c.Start(ctx); err != nil {
			errs <- err
			defer inspectRefresh()
			return
		}
		t.Log().Debugf("started")
		if t.Detach {
			// t.c.Wait(ctx, WaitConditionRunning) return err not found
			// use check running instead
			t.Log().Infof("check running")
			inspect, err := t.c.InspectRefresh(context.Background())
			if err != nil {
				err = fmt.Errorf("check running: can't inspect: %s", err)
			} else if inspect == nil {
				err = fmt.Errorf("check running: inspect is nil")
			} else if inspect.Running() {
				t.Log().Debugf("check running: ok")
			} else {
				err = fmt.Errorf("check running: false")
			}
			if err != nil {
				t.Log().Warnf("%s", err)
			}
			errs <- err
			return
		}
		defer inspectRefresh()
		t.Log().Infof("wait not running")
		if ok, err := t.c.Wait(ctx, WaitConditionNotRunning); err != nil {
			t.Log().Debugf("wait not running: %s", err)
			errs <- nil
			return
		} else {
			t.Log().Debugf("wait not running: %v", ok)
			errs <- nil
			return
		}
	}()

	var timerC <-chan time.Time
	if t.StartTimeout != nil && *t.StartTimeout > 0 {
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
			return nil
		}
		err = fmt.Errorf("container start %s (%s): %s", name, id, err)
		t.Log().Errorf("%s", err)
		return err
	case <-timerC:
		err := fmt.Errorf("container start %s (%s): timeout", name, id)
		t.Log().Errorf("%s", err)
		return err
	}
}
