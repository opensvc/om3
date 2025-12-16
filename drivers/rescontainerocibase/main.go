// Package rescontainerocibase provides base settings for to implement resource
// container oci drivers.
//
// It Defines BT that may help container oci composition for resource container
// oci driver interface.
//
// It Defines Executor that implements Executer interface.
//
// It Defines ExecutorArg that implements ExecutorArgser interface.
package rescontainerocibase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/kballard/go-shellquote"
	"golang.org/x/sys/unix"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/vpath"
	"github.com/opensvc/om3/v3/util/args"
	"github.com/opensvc/om3/v3/util/envprovider"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/pg"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/stringslice"
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
		ReadOnly        string         `json:"read_only"`
		RegistryCreds   string         `json:"registry_creds"`
		PullTimeout     *time.Duration `json:"pull_timeout"`
		StartTimeout    *time.Duration `json:"start_timeout"`
		StopTimeout     *time.Duration `json:"stop_timeout"`
		LogOutputs      bool           `json:"log_outputs"`

		executer   Executer
		xContainer map[string]containerNamer
	}

	BindMount struct {
		Source string
		Target string
		Option string
	}
)

type (
	// ExecuteContainer interface defines the functions used to manage container
	// lifecycle.
	ExecuteContainer interface {
		Enter(context.Context) error
		Start(context.Context) error
		Stop(context.Context) error
		Remove(context.Context) error
		Run(context.Context) error
	}

	ExecuteEncaper interface {
		EncapCp(context.Context, string, string) error
		EncapCmd(context.Context, []string, []string, io.Reader) (*exec.Cmd, error)
	}

	// ExecuteImager interface defines the functions used to manage container
	// image lifecycle.
	ExecuteImager interface {
		HasImage(context.Context) (bool, string, error)
		Pull(context.Context) error
	}

	// ExecuteInspecter interface defines the functions used to retrieve container
	// inspecter.
	ExecuteInspecter interface {
		Inspect(context.Context) (Inspecter, error)
		InspectRefresh(context.Context) (Inspecter, error)
	}

	// ExecuteWaiter interface defines the functions used to manage container
	// wait functions.
	ExecuteWaiter interface {
		WaitNotRunning(context.Context) error
		WaitRemoved(context.Context) error
	}

	// Executer defines interfaces for container operations. It must be
	// implemented by container executors.
	Executer interface {
		ExecuteContainer
		ExecuteImager
		ExecuteInspecter
		ExecuteWaiter
		ExecuteEncaper
	}

	// ExecutorBaseArgser is an optional interface executor may implement to
	// add base args to all doExecRun commands.
	ExecutorBaseArgser interface {
		ExecBaseArgs() []string
	}

	// ExecutorContainerArgser defines interfaces functions that provides
	// args for container resource operations.
	ExecutorContainerArgser interface {
		CpCmdArgs(string, string) []string
		EnterCmdArgs(pid int) []string
		EnterCmdCheckArgs(pid int) []string
		ExecCmdArgs([]string, []string, bool) []string
		RemoveArgs() *args.T
		RunArgsBase(ctx context.Context) (*args.T, error)
		RunArgsImage() (*args.T, error)
		RunArgsCommand() (*args.T, error)
		RunCmdEnv() (map[string]string, error)
		StartArgs(ctx context.Context) (*args.T, error)
		StopArgs() *args.T
	}

	// ExecutorInspectArgser defines interfaces functions that provides
	// args for container resource inspect operations.
	ExecutorInspectArgser interface {
		HasImageArgs() *args.T
		InspectArgs() *args.T
		InspectParser([]byte) (Inspecter, error)
	}

	// ExecutorImageArgser defines interfaces functions that provides args for
	// image operations.
	ExecutorImageArgser interface {
		PullArgs() *args.T
	}

	// ExecutorArgser defines interfaces for container executor args.
	// The ExecutorArgser interface is meant to define the required arguments
	// or methods that a container executor should have, focusing on resource
	// information. These arguments are used by executors to manage containers.
	ExecutorArgser interface {
		ExecutorContainerArgser
		ExecutorImageArgser
		ExecutorInspectArgser
		ExecuteWaiter
	}

	// Inspecter defines interfaces functions that a container inspector must
	// provide.
	Inspecter interface {
		Config() *InspectDataConfig
		Defined() bool
		ID() string
		ImageID() string
		HostConfig() *InspectDataHostConfig
		ExitCode() int
		PID() int
		Running() bool
		SandboxKey() string
		Status() string
	}

	Logger interface {
		Log() *plog.Logger
	}
)

type (
	ExecuterGetter interface {
		Executer() Executer
	}

	ExecutorArgserGetter interface {
		ExecutorArgser() ExecutorArgser
	}
)

// defines used internal interfaces
type (
	containerNamer interface {
		ContainerName() string
	}

	containerIDer interface {
		ContainerID(ctx context.Context) string
	}

	containerInspectRefresher interface {
		ContainerInspectRefresh(context.Context) (Inspecter, error)
	}
)

const (
	imagePullPolicyAlways = "always"
	imagePullPolicyOnce   = "once"
)

var _ resource.Encaper = (*BT)(nil)

func (t *BT) Configure() error {
	l := t.T.Log().Attr("container_name", t.ContainerName())
	t.SetLoggerForTest(l)
	if !t.Detach {
		t.Remove = true
	}
	return nil
}

func (t *BT) IsAlwaysImagePullPolicy() bool {
	return t.ImagePullPolicy == imagePullPolicyAlways
}

// ContainerID returns the container inspect ID, or ""
func (t *BT) ContainerID(ctx context.Context) string {
	if t.executer == nil {
		t.Log().Tracef("can't get container id from undefined executer")
		return ""
	}
	if i, err := t.executer.Inspect(ctx); err != nil {
		return ""
	} else if i == nil {
		return ""
	} else {
		return i.ID()
	}
}

// Implement the resource.Hostnamer interface,
// used by ip resources to obtain a hostname-based DNS record
func (t *BT) GetHostname() string {
	return t.Hostname
}

// ContainerName formats a docker container name
func (t *BT) ContainerName() string {
	if t.Name != "" {
		return t.Name
	}
	var s string
	switch t.Path.Namespace {
	case naming.NsRoot, "":
		s = ""
	default:
		s = t.Path.Namespace + ".."
	}
	s = s + t.Path.Name + "." + strings.ReplaceAll(t.ResourceID.String(), "#", ".")
	return s
}

func (t *BT) ContainerInspect(ctx context.Context) (Inspecter, error) {
	if t.executer == nil {
		return nil, errors.New("can't get inspect from undefined executer")
	}
	return t.executer.Inspect(ctx)
}

func (t *BT) ContainerInspectRefresh(ctx context.Context) (Inspecter, error) {
	if t.executer == nil {
		return nil, errors.New("can't get refresh inspect from undefined executer")
	}
	return t.executer.InspectRefresh(ctx)
}

func (t *BT) Enter(ctx context.Context) error {
	if t.executer == nil {
		return t.logMainAction("enter", errors.New("undefined executer"))
	}
	return t.executer.Enter(ctx)
}

func (t *BT) FormatNS(s string) (string, error) {
	switch s {
	case "", "none", "host":
		return s, nil
	}
	if !strings.HasPrefix(s, "container#") {
		// "", "none", "container:..."
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

// GenEnv returns the list of environment variables from the resource/object and
// its ConfigsEnv: []string{"PUBLICVAR1=Value", ...}
// secret var names from its SecretsEnv are added to the list: "SECRETVAR1", "SECRETVAR2",...
// values for secrets are added to the returned envM: {"SECRETVAR1":"SECRETVALUE1", ...}
// It may be used by executorArgser to prepare run args and run command environment.
func (t *BT) GenEnv(ctx context.Context) (envL []string, envM map[string]string, err error) {
	envM = make(map[string]string)
	envL = []string{
		"OPENSVC_RID=" + t.RID(),
		"OPENSVC_NAME=" + t.Path.String(),
		"OPENSVC_KIND=" + t.Path.Kind.String(),
		"OPENSVC_ID=" + t.ObjectID.String(),
		"OPENSVC_NAMESPACE=" + t.Path.Namespace,
	}
	if len(t.Env) > 0 {
		envL = append(envL, t.Env...)
	}
	if tempEnv, err := envprovider.From(t.ConfigsEnv, t.Path.Namespace, "cfg"); err != nil {
		return nil, nil, err
	} else {
		envL = append(envL, tempEnv...)
	}
	if tempEnv, err := envprovider.From(t.SecretsEnv, t.Path.Namespace, "sec"); err != nil {
		return nil, nil, err
	} else {
		for _, s := range tempEnv {
			kv := strings.SplitN(s, "=", 2)
			if len(kv) != 2 {
				return nil, nil, fmt.Errorf("can't prepare env from secrets")
			}
			envM[kv[0]] = kv[1]
			envL = append(envL, kv[0])
		}
	}

	// add environment variables from the "task run" commandline (--env)
	envL = append(envL, actioncontext.Env(ctx)...)

	return envL, envM, nil
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *BT) Label(_ context.Context) string {
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

func (t *BT) LinkNames() []string {
	return []string{t.RID()}
}

func (t *BT) Mounts(ctx context.Context) ([]BindMount, error) {
	mounts := make([]BindMount, 0)
	for _, s := range t.VolumeMounts {
		var source, target, opt string
		l := strings.Split(s, ":")
		n := len(l)
		switch n {
		case 2:
			source = l[0]
			target = l[1]
			opt = "rw"
		case 3:
			source = l[0]
			target = l[1]
			opt = l[2]
		default:
			return mounts, fmt.Errorf("invalid volumes_mount entry: %s: 1-2 column-characters allowed", s)
		}
		if len(source) == 0 {
			return mounts, fmt.Errorf("invalid volumes_mount entry: %s: empty source", s)
		}
		if len(target) == 0 {
			return mounts, fmt.Errorf("invalid volumes_mount entry: %s: empty target", s)
		}
		if strings.HasPrefix(source, "/") {
			// pass
		} else if srcRealpath, vol, err := vpath.HostPathAndVol(ctx, source, t.Path.Namespace); err != nil {
			return mounts, err
		} else if file.IsProtected(srcRealpath) {
			return mounts, fmt.Errorf("invalid volumes_mount entry: %s: expanded to the protected path %s", s, srcRealpath)
		} else {
			source = srcRealpath

			if newOpt, err := mangleVolMountOptions(opt, vol); err != nil {
				return mounts, fmt.Errorf("can't prepare volume options for volume mount '%s': %w", s, err)
			} else {
				opt = newOpt
			}
		}

		mounts = append(mounts, BindMount{Source: source, Target: target, Option: opt})
	}
	return mounts, nil
}

// NeedPreStartRemove return true when container has Remove or not Detach.
// During Start existing container (with Remove true or Detach false) must be removed,
func (t *BT) NeedPreStartRemove() bool {
	return t.Remove || !t.Detach
}

// NetNSPath returns the container inspect SandboxKey or ""
func (t *BT) NetNSPath(ctx context.Context) (string, error) {
	if t.executer == nil {
		return "", fmt.Errorf("NetNSPath: undefined executer")
	}
	if i, err := t.executer.Inspect(ctx); err != nil {
		return "", nil
	} else if i == nil {
		return "", nil
	} else {
		return i.SandboxKey(), nil
	}
}

// PID returns the container inspect PID or 0
func (t *BT) PID(ctx context.Context) int {
	if t.executer == nil {
		t.Log().Tracef("PID called with undefined executer")
		return 0
	}
	if i, err := t.executer.Inspect(ctx); err != nil {
		t.Log().Tracef("PID inspect: %s", err)
		return 0
	} else if i == nil {
		return 0
	} else {
		return i.PID()
	}
}

func (t *BT) Provision(_ context.Context) error {
	return nil
}

func (t *BT) Provisioned(ctx context.Context) (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

// Signal implements object.Signaler
func (t *BT) Signal(ctx context.Context, sig syscall.Signal) error {
	name := t.ContainerName()
	if t.executer == nil {
		return fmt.Errorf("signal: undefined executer")
	}
	inspect, err := t.executer.InspectRefresh(ctx)
	if err != nil {
		t.Log().Errorf("signal: inspect refresh container %s: %s", name, err)
		return err
	} else if inspect == nil {
		t.Log().Infof("skip signal: container %s not found", name)
		return nil
	}
	if !inspect.Running() {
		t.Log().Infof("skip signal: container %s not running", name)
		return nil
	}
	pid := inspect.PID()
	if pid == 0 {
		t.Log().Infof("skip signal: can't detect container %s pid", name)
	}
	t.Log().Infof("send %s signal to container %s (pid %d)", unix.SignalName(sig), name, pid)
	return syscall.Kill(pid, sig)
}

func (t *BT) Start(ctx context.Context) error {
	name := t.ContainerName()
	log := t.Log()

	logError := func(err error) error {
		return t.logMainAction("start", err)
	}

	if t.executer == nil {
		return t.logMainAction("start", errors.New("undefined executer"))
	}

	if err := t.ApplyPGChain(ctx); err != nil {
		return err
	}

	inspect, err := t.executer.Inspect(ctx)
	if err != nil {
		return t.logMainAction("start", fmt.Errorf("inspect: %s", err))
	} else if inspect == nil || !inspect.Defined() {
		return logError(t.pullAndRun(ctx))
	} else if inspect.Running() {
		log.Infof("container start %s: already started", name)
		return nil
	} else {
		// it is defined
		inspectStatus := inspect.Status()
		log.Tracef("container start %s: defined with inspectStatus %s", name, inspectStatus)
		if t.NeedPreStartRemove() {
			log.Infof("container start %s: remove leftover container", name)
			if err := t.executer.Remove(ctx); err != nil {
				return logError(err)
			}
			return logError(t.pullAndRun(ctx))
		} else if inspectStatus == "initialized" {
			log.Infof("container inspectStatus %s, try fix with stop first", inspectStatus)
			if err := t.executer.Stop(ctx); err != nil {
				return err
			}
			return logError(t.findAndStart(ctx))
		} else {
			log.Infof("container inspectStatus %s", inspectStatus)
			return logError(t.findAndStart(ctx))
		}
	}
}

func (t *BT) Stop(ctx context.Context) error {
	name := t.ContainerName()
	log := t.Log()

	logError := func(err error) error {
		return t.logMainAction(fmt.Sprintf("container stop %s:", t.RID()), err)
	}

	if t.executer == nil {
		return t.logMainAction("stop", errors.New("undefined executer"))
	}

	inspect, err := t.executer.InspectRefresh(ctx)
	if err != nil {
		return t.logMainAction("stop", fmt.Errorf("can't refresh inspect: %s", err))
	}
	if inspect == nil {
		log.Infof("already stopped")
		return nil
	}

	if inspect.Running() {
		defer func() {
			_, _ = t.executer.InspectRefresh(ctx)
		}()
		if err := t.executer.Stop(ctx); err != nil {
			t.Log().Errorf("stop: %s", err)
			return err
		}
		log.Tracef("container stopped")
	}

	if t.Remove {
		if hostConfig := inspect.HostConfig(); hostConfig != nil && !hostConfig.AutoRemove {
			t.Log().Tracef("remove container %s", name)
			if err := t.executer.Remove(ctx); err != nil {
				return logError(fmt.Errorf("can't remove container %s", name))
			}
		}
		t.Log().Tracef("wait removed condition")
		if err := t.executer.WaitRemoved(ctx); err != nil {
			t.Log().Warnf("wait removed: %s", err)
			return err
		} else {
			t.Log().Tracef("removed")
			return nil
		}
	} else {
		t.Log().Tracef("wait not running condition")
		if err := t.executer.WaitNotRunning(ctx); err != nil {
			t.Log().Warnf("wait not running: %s", err)
			return err
		}
		t.Log().Tracef("wait not running: done")
	}
	return nil
}

func (t *BT) Status(ctx context.Context) status.T {
	if !t.Detach {
		t.Log().Tracef("status n/a on not detach")
		return status.NotApplicable
	}

	t.Log().Tracef("Status.enter")
	defer t.Log().Tracef("Status.return")
	if t.executer == nil {
		t.Log().Tracef("status n/a on undefined executer")
		return status.NotApplicable
	}
	inspect, err := t.executer.Inspect(ctx)
	if err != nil {
		t.Log().Tracef("status down on inspect: %s", err)
		return status.Down
	} else if inspect == nil {
		t.Log().Tracef("status down on inspect nil")
		return status.Down
	}
	if inspectConfig := inspect.Config(); inspectConfig != nil {
		if t.Hostname != "" && inspectConfig.Hostname != t.Hostname {
			t.warnAttrDiff("hostname", inspectConfig.Hostname, t.Hostname)
		}
		if inspectConfig.OpenStdin != t.Interactive {
			t.warnAttrDiff("interactive", fmt.Sprint(inspectConfig.OpenStdin), fmt.Sprint(t.Interactive))
		}
		if len(t.Entrypoint) > 0 && !stringslice.Equal(inspectConfig.Entrypoint, t.Entrypoint) {
			t.warnAttrDiff("entrypoint", shellquote.Join(inspectConfig.Entrypoint...), shellquote.Join(t.Entrypoint...))
		}
		if inspectConfig.Tty != t.TTY {
			t.warnAttrDiff("tty", fmt.Sprint(inspectConfig.Tty), fmt.Sprint(t.TTY))
		}
	}
	if inspectHostConfig := inspect.HostConfig(); inspectHostConfig != nil {
		if inspectHostConfig.Privileged != t.Privileged {
			t.warnAttrDiff("privileged", fmt.Sprint(inspectHostConfig.Privileged), fmt.Sprint(t.Privileged))
		}
		if t.ReadOnly != "" && fmt.Sprint(inspectHostConfig.ReadonlyRootfs) != t.ReadOnly {
			t.warnAttrDiff("read_only", fmt.Sprint(inspectHostConfig.ReadonlyRootfs), t.ReadOnly)
		}
		t.statusInspectNS(ctx, "netns", inspectHostConfig.NetworkMode, t.NetNS)
		t.statusInspectNS(ctx, "pidns", inspectHostConfig.PidMode, t.PIDNS)
		t.statusInspectNS(ctx, "ipcns", inspectHostConfig.IpcMode, t.IPCNS)
		t.statusInspectNS(ctx, "utsns", inspectHostConfig.UTSMode, t.UTSNS)
	}

	if _, imageID, err := t.executer.HasImage(ctx); err == nil {
		containerImageID := inspect.ImageID()
		if containerImageID != imageID {
			t.warnAttrDiff("image "+t.Image, containerImageID, imageID)
		}
	}

	if !inspect.Running() {
		if t.Remove {
			t.StatusLog().Warn("not removed container")
			return status.Down
		}
		return status.Down
	}
	return status.Up
}

func (t *BT) Unprovision(_ context.Context) error {
	return nil
}

func (t *BT) WithExecuter(c Executer) *BT {
	t.executer = c
	return t
}

// Executer implements ExecuterGetter for external tests
func (t *BT) Executer() Executer {
	return t.executer
}

func (t *BT) containerLabelID() string {
	return fmt.Sprintf("%s.%s", t.ObjectID, t.ResourceID.String())
}

func (t *BT) findAndStart(ctx context.Context) error {
	var id string
	name := t.ContainerName()
	if t.executer == nil {
		return fmt.Errorf("findAndStart: undefined executer")
	}
	if i, err := t.executer.Inspect(ctx); err != nil {
		id = ""
	} else {
		id = i.ID()
	}
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
			_, err := t.executer.InspectRefresh(context.Background())
			if err != nil {
				t.Log().Warnf("findAndStart InspectRefresh: %s", err)
			}
		}

		if err := t.executer.Start(ctx); err != nil {
			errs <- err
			defer inspectRefresh()
			return
		}
		t.Log().Tracef("started")
		if t.Detach {
			// t.executer.Wait(ctx, WaitConditionRunning) return err not found
			// use check running instead
			t.Log().Infof("check running")
			inspect, err := t.executer.InspectRefresh(context.Background())
			if err != nil {
				err = fmt.Errorf("check running: can't inspect: %s", err)
			} else if inspect == nil {
				err = fmt.Errorf("check running: inspect is nil")
			} else if inspect.Running() {
				t.Log().Tracef("check running: ok")
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
		if err := t.executer.WaitNotRunning(ctx); err != nil {
			t.Log().Tracef("wait not running: %s", err)
			errs <- nil
			return
		} else {
			t.Log().Tracef("wait not running: done")
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
			actionrollback.Register(ctx, func(ctx context.Context) error {
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

func (t *BT) logMainAction(s string, err error) error {
	if err != nil {
		err = fmt.Errorf("%s: %s", s, err)
		t.Log().Errorf("%s", err)
		return err
	}
	return nil
}

func (t *BT) pull(ctx context.Context) error {
	if t.executer == nil {
		return fmt.Errorf("pull: undefined executer")
	}
	if t.PullTimeout != nil && *t.PullTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *t.PullTimeout)
		defer cancel()
	}
	if err := t.executer.Pull(ctx); err != nil {
		return fmt.Errorf("can't pull image %s: %s", t.Image, err)
	}
	return nil
}

func (t *BT) pullAndRun(ctx context.Context) error {
	if t.executer == nil {
		return fmt.Errorf("pullAndRun: undefined executer")
	}
	if t.IsAlwaysImagePullPolicy() {
		t.Log().Tracef("container start: with image policy: always")
		if err := t.pull(ctx); err != nil {
			return err
		}
	} else if hasImage, _, err := t.executer.HasImage(ctx); err != nil {
		return fmt.Errorf("unable to detect if image %s exists locally: %s", t.Image, err)
	} else if !hasImage {
		if err := t.pull(ctx); err != nil {
			return err
		}
	}
	refreshCtx := ctx

	if t.StartTimeout != nil && *t.StartTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *t.StartTimeout)
		defer cancel()
	}

	defer func() {
		_, _ = t.executer.InspectRefresh(refreshCtx)
	}()

	return t.executer.Run(ctx)
}

func (t *BT) statusInspectNS(ctx context.Context, attr, current, target string) {
	switch target {
	case "":
		return
	case "none", "host":
		if current != target {
			t.warnAttrDiff(attr, current, target)
		}
		return
	}
	rid, err := resourceid.Parse(target)
	if err != nil {
		t.StatusLog().Warn("%s: invalid value %s (must be none, host or container#<n>)", attr, target)
		return
	}

	if t.xContainer == nil {
		t.xContainer = make(map[string]containerNamer)
	}

	tgt, ok := t.xContainer[rid.String()]
	if !ok {
		if r := t.GetObjectDriver().ResourceByID(rid.String()); r == nil {
			t.StatusLog().Warn("%s: %s resource not found", attr, target)
			return
		} else if tgt, ok = r.(containerNamer); !ok {
			t.StatusLog().Warn("%s resource %s is not a container namer", attr, target)
			return
		} else {
			if r, ok := r.(containerInspectRefresher); ok {
				if _, err := r.ContainerInspectRefresh(ctx); err != nil {
					t.StatusLog().Warn("%s resource %s inspect error", attr, target)
				}
			}
			t.xContainer[rid.String()] = tgt
		}
	}

	var tgtName, tgtID string
	if tgt == nil {
		t.StatusLog().Warn("%s: %s resource not found", attr, target)
		return
	} else {
		tgtName = "container:" + tgt.ContainerName()
		if i, ok := tgt.(containerIDer); ok {
			tgtID = "container:" + i.ContainerID(ctx)
		}
	}

	switch {
	case tgtName == current:
		t.Log().Tracef("valid %s cross-resource reference to %s: %s", attr, tgtName, current)
	case tgtID == current:
		t.Log().Tracef("valid %s cross-resource reference to %s: %s", attr, tgtID, current)
	default:
		t.Log().Tracef("invalid %s cross-resource reference to %s: found %s instead of %s or %s",
			attr, target, current, tgtName, tgtID)
		t.warnAttrDiff(attr, current, tgtName)
	}
}

func (t *BT) warnAttrDiff(attr, current, target string) {
	t.StatusLog().Warn("%s is %s, should be %s", attr, current, target)
}

// mangleVolMountOptions mangles volume initialOptions to return options
// with "rw" or "ro" option.
//
//		  if volume access is read only or initial options contains "ro"
//	   then options will contain "ro"
//	   else options will contain "rw" if volume access is read only or initial options contains "ro":
func mangleVolMountOptions(initialOptions string, vol object.Vol) (string, error) {
	opts := strings.Split(initialOptions, ",")

	// wantsRo rule: opts contains "ro" or volume access is read only
	wantsRo := slices.Contains(opts, "ro")

	if !wantsRo && vol != nil {
		if volAccess, err := vol.Access(); err != nil {
			return "", err
		} else {
			wantsRo = volAccess.IsReadOnly()
		}
	}

	var newOpts []string
	if wantsRo {
		newOpts = append(newOpts, "ro")
	} else {
		newOpts = append(newOpts, "rw")
	}
	for _, o := range opts {
		switch o {
		case "ro", "rw":
		case "": // from strings.Split when initial opt is ""
		default:
			newOpts = append(newOpts, o)
		}
	}

	return strings.Join(newOpts, ","), nil
}

func (t *BT) EncapCmd(ctx context.Context, args []string, env []string, stdin io.Reader) (resource.Commander, error) {
	return t.executer.EncapCmd(ctx, args, env, stdin)
}

func (t *BT) EncapCp(ctx context.Context, src, dst string) error {
	return t.executer.EncapCp(ctx, src, dst)
}

func (t *BT) GetOsvcRootPath() string {
	if t.OsvcRootPath != "" {
		return filepath.Join(t.OsvcRootPath, "bin", "om")
	}
	return filepath.Join(rawconfig.Paths.Bin, "om")
}
