package rescontainerdocker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cpuguy83/go-docker"
	"github.com/cpuguy83/go-docker/container"
	"github.com/cpuguy83/go-docker/container/containerapi"
	"github.com/cpuguy83/go-docker/container/containerapi/mount"
	"github.com/cpuguy83/go-docker/errdefs"
	"github.com/cpuguy83/go-docker/image"
	"github.com/cpuguy83/go-docker/image/imageapi"
	"github.com/google/uuid"
	"github.com/kballard/go-shellquote"
	"golang.org/x/sys/unix"

	"opensvc.com/opensvc/core/actionrollback"
	"opensvc.com/opensvc/core/fqdn"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/resourceid"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/envprovider"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/pg"
	"opensvc.com/opensvc/util/stringslice"
)

const (
	AlwaysPolicy = "always"
	OncePolicy   = "once"
)

type (
	T struct {
		resource.T
		PG              pg.Config      `json:"pg"`
		Path            path.T         `json:"path"`
		ObjectID        uuid.UUID      `json:"object_id"`
		SCSIReserv      bool           `json:"scsireserv"`
		PromoteRW       bool           `json:"promote_rw"`
		NoPreemptAbort  bool           `json:"NoPreemptAbort"`
		OsvcRootPath    string         `json:"osvc_root_path"`
		GuestOS         string         `json:"guest_os"`
		Name            string         `json:"name"`
		Hostname        string         `json:"hostname"`
		Image           string         `json:"image"`
		ImagePullPolicy string         `json:"image_pull_policy"`
		CWD             string         `json:"cwd"`
		Command         []string       `json:"command"`
		DNS             []string       `json:"dns"`
		DNSSearch       []string       `json:"dns_search"`
		RunArgs         []string       `json:"run_args"`
		Entrypoint      []string       `json:"entrypoint"`
		Detach          bool           `json:"detach"`
		Remove          bool           `json:"remove"`
		Privileged      bool           `json:"privileged"`
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
	}

	containerNamer interface {
		ContainerName() string
	}

	imageCacheMap struct {
		m  map[string]*image.Image
		mu sync.Mutex
	}
)

var (
	// Allocate a single client socket for all container.docker resources
	// Get/Init it via cli()
	clientCache *docker.Client
	imageCache  = NewImageCacheMap()
)

func NewImageCacheMap() *imageCacheMap {
	return &imageCacheMap{
		m: make(map[string]*image.Image),
	}
}

func (t imageCacheMap) Get(name string) (*image.Image, bool) {
	t.mu.Lock()
	img, ok := t.m[name]
	t.mu.Unlock()
	return img, ok
}

func (t *imageCacheMap) Put(name string, img *image.Image) {
	t.mu.Lock()
	t.m[name] = img
	t.mu.Unlock()
}

func cli() *docker.Client {
	if clientCache != nil {
		return clientCache
	}
	clientCache = docker.NewClient()
	return clientCache
}

func New() resource.Driver {
	t := &T{}
	return t
}

func parseImage(s string) (repo string, img string, tag string, err error) {
	l := strings.SplitN(s, "/", 2)
	switch len(l) {
	case 1:
		repo = "dockerhub.io"
		s = l[0]
	case 2:
		repo = l[0]
		s = l[1]
	default:
		err = fmt.Errorf("image must contain 0 or 1 slash")
		return
	}
	l = strings.SplitN(s, ":", 2)
	switch len(l) {
	case 1:
		img = l[0]
		tag = "latest"
	case 2:
		img = l[0]
		tag = l[1]
	default:
		err = fmt.Errorf("image must contain 0 or 1 column")
		return
	}
	return
}

func (t T) pull(ctx context.Context) error {
	repo, img, tag, err := parseImage(t.Image)
	if err != nil {
		return err
	}
	t.Log().Info().
		Str("repo", repo).
		Str("image", img).
		Str("tag", tag).
		Msg("pull image")
	_, err = cli().ImageService().Pull(ctx, func(config *image.PullConfig) {
		config.Image = img
		config.Tag = tag
		config.Repo = repo
	})
	return err
}

func (t T) labels() (map[string]string, error) {
	data := make(map[string]string)
	data["com.opensvc.id"] = t.containerLabelID()
	data["com.opensvc.path"] = t.Path.String()
	data["com.opensvc.namespace"] = t.Path.Namespace
	data["com.opensvc.kind"] = t.Path.Kind.String()
	data["com.opensvc.name"] = t.Path.Name
	data["com.opensvc.rid"] = t.ResourceID.String()
	return data, nil
}

func (t T) mounts() ([]mount.Mount, error) {
	mounts := make([]mount.Mount, 0)
	for _, s := range t.VolumeMounts {
		l := strings.Split(s, ":")
		n := len(l)
		m := mount.Mount{
			Type:        mount.TypeBind,
			Consistency: mount.ConsistencyDefault,
		}
		var opt string
		switch n {
		case 2:
			m.Source = l[0]
			m.Target = l[1]
			opt = "rw"
		case 3:
			m.Source = l[0]
			m.Target = l[1]
			opt = l[2]
		default:
			return mounts, fmt.Errorf("invalid volumes_mount entry: %s: 1-2 column-characters allowed", s)
		}
		optM := make(map[string]interface{})
		for _, e := range strings.Split(opt, ",") {
			optM[e] = nil
		}
		if _, ok := optM["ro"]; ok {
			m.ReadOnly = true
		}
		if len(m.Source) == 0 {
			return mounts, fmt.Errorf("invalid volumes_mount entry: %s: empty source", s)
		}
		if len(m.Target) == 0 {
			return mounts, fmt.Errorf("invalid volumes_mount entry: %s: empty target", s)
		}
		if srcRealpath, err := object.Realpath(m.Source, t.Path.Namespace); err != nil {
			return mounts, err
		} else {
			m.Source = srcRealpath
		}

		mounts = append(mounts, m)
	}
	return mounts, nil
}

func (t T) devices() ([]containerapi.DeviceMapping, error) {
	data := make([]containerapi.DeviceMapping, 0)
	for _, s := range t.Devices {
		l := strings.Split(s, ":")
		dm := containerapi.DeviceMapping{}
		n := len(l)
		switch {
		case n <= 3:
			dm.PathOnHost = l[0]
			dm.PathInContainer = l[1]
			fallthrough
		case n == 3:
			dm.CgroupPermissions = l[2]
		}
		data = append(data, dm)
	}
	return data, nil
}

func (t T) Start(ctx context.Context) error {
	cs := cli().ContainerService()
	name := t.ContainerName()
	inspect, err := cs.Inspect(ctx, name)
	if err == nil {
		if inspect.State.Running {
			t.Log().Info().Msg("already running")
			return nil
		} else {
			if t.needRemove() {
				t.Log().Info().Str("name", name).Msgf("remove leftover container")
				if err := cs.Remove(ctx, name); err != nil {
					return err
				}
				if t.ImagePullPolicy == AlwaysPolicy {
					if err := t.pull(ctx); err != nil {
						return err
					}
				}
				c, err := t.create(ctx)
				if err != nil {
					return err
				}
				return t.start(ctx, c)
			} else {
				t.Log().Info().Str("name", name).Str("id", inspect.ID).Msg("start container")
				c := cs.NewContainer(ctx, inspect.ID)
				return t.start(ctx, c)
			}
		}
	} else {
		if t.ImagePullPolicy == AlwaysPolicy {
			if err := t.pull(ctx); err != nil {
				return err
			}
		} else if _, err = t.imageInspect(); err != nil {
			if err := t.pull(ctx); err != nil {
				return err
			}
		}
		c, err := t.create(ctx)
		if err != nil {
			return err
		}
		return t.start(ctx, c)
	}
	return nil
}

func (t T) start(ctx context.Context, c *container.Container) error {
	errs := make(chan error, 1)
	go func() {
		t.Log().Info().Msgf("start container (timeout %s)", t.StartTimeout)
		if err := c.Start(ctx); err != nil {
			errs <- err
			return
		}
		if t.Detach {
			errs <- nil
			return
		}
		_, err := c.Wait(ctx, container.WithWaitCondition(container.WaitConditionNotRunning))
		errs <- err
	}()
	select {
	case err := <-errs:
		return err
	case <-time.After(*t.StartTimeout):
		return fmt.Errorf("timeout")
	}
	return nil
}

func (t T) create(ctx context.Context) (*container.Container, error) {
	var (
		env     []string
		labels  map[string]string
		devices []containerapi.DeviceMapping
		mounts  []mount.Mount
		err     error
		inspect imageapi.ImageInspect
	)
	if inspect, err = t.imageInspect(); err != nil {
		return nil, err
	}
	if env, err = t.env(); err != nil {
		return nil, err
	}
	if labels, err = t.labels(); err != nil {
		return nil, err
	}
	if devices, err = t.devices(); err != nil {
		return nil, err
	}
	if mounts, err = t.mounts(); err != nil {
		return nil, err
	}

	config := containerapi.Config{
		Hostname:    t.hostname(),
		Tty:         t.TTY,
		Env:         env,
		Cmd:         t.command(inspect),
		Entrypoint:  t.entrypoint(inspect),
		Image:       t.Image,
		WorkingDir:  t.CWD,
		Labels:      labels,
		OpenStdin:   t.Interactive,
		StopTimeout: t.stopTimeout(),
	}

	hostConfig := containerapi.HostConfig{}
	hostConfig.Privileged = t.Privileged
	hostConfig.AutoRemove = t.needRemove()
	hostConfig.Cgroup = t.PG.ID
	hostConfig.Devices = devices
	hostConfig.Mounts = mounts
	hostConfig.DNS = t.dns()
	hostConfig.DNSOptions = t.dnsOptions()
	hostConfig.DNSSearch = t.dnsSearch()
	if hostConfig.NetworkMode, err = t.formatNS(t.NetNS); err != nil {
		return nil, err
	}
	if hostConfig.PidMode, err = t.formatNS(t.PIDNS); err != nil {
		return nil, err
	}
	if hostConfig.IpcMode, err = t.formatNS(t.IPCNS); err != nil {
		return nil, err
	}
	if hostConfig.UTSMode, err = t.formatNS(t.UTSNS); err != nil {
		return nil, err
	}
	if hostConfig.UsernsMode, err = t.formatNS(t.UserNS); err != nil {
		return nil, err
	}

	name := t.ContainerName()

	configObf := config
	if configObf.Env, err = t.obfuscatedEnv(); err != nil {
		return nil, err
	}
	configStr, _ := json.Marshal(configObf)
	hostConfigStr, _ := json.Marshal(hostConfig)

	// create missing mount sources
	for _, m := range mounts {
		if file.Exists(m.Source) {
			continue
		}
		t.Log().Info().Str("path", m.Source).Msg("create missing mount source")
		if err := os.MkdirAll(m.Source, os.ModePerm); err != nil {
			return nil, err
		}
	}

	t.Log().Info().
		Str("name", name).
		Bytes("config", configStr).
		Bytes("hostConfig", hostConfigStr).
		Msg("create container")
	c, err := cli().ContainerService().Create(
		ctx,
		container.WithCreateName(name),
		container.WithCreateConfig(config),
		container.WithCreateHostConfig(hostConfig),
	)
	if err != nil {
		return nil, err
	}
	actionrollback.Register(ctx, func() error {
		var xc int
		if err := c.Stop(ctx); err != nil {
			return err
		}
		if x, err := c.Wait(ctx, container.WithWaitCondition(container.WaitConditionNotRunning)); err != nil {
			return err
		} else {
			xc = x.ExitCode()
		}
		t.Log().Info().Msgf("exited with code %d", xc)
		return nil
	})
	return c, nil
}

func (t T) Stop(ctx context.Context) error {
	name := t.ContainerName()
	inspect, err := cli().ContainerService().Inspect(ctx, name)
	c := cli().ContainerService().NewContainer(ctx, inspect.ID)
	if (err == nil && !inspect.State.Running) || errdefs.IsNotFound(err) {
		t.Log().Info().Str("name", name).Msg("already stopped")
	} else {
		t.Log().Info().Str("name", name).Str("id", inspect.ID).Msg("stop container")
		if err := c.Stop(ctx); err != nil {
			return err
		}
	}
	if t.needRemove() && !errdefs.IsNotFound(err) {
		if !inspect.HostConfig.AutoRemove {
			t.Log().Info().Str("name", name).Msg("remove container")
			return cli().ContainerService().Remove(ctx, name)
		}
		xs, err := c.Wait(ctx, container.WithWaitCondition(container.WaitConditionRemoved))
		if err != nil {
			return err
		}
		t.Log().Debug().Msgf("wait removed condition ended with exit code %d", xs.ExitCode())
	} else {
		t.Log().Info().Msg("already removed")
	}
	return nil
}

func (t *T) warnAttrDiff(attr, current, target string) {
	t.StatusLog().Warn("%s is %s, should be %s", attr, current, target)
}

// NetNSPath implements the resource.NetNSPather optional interface.
// Used by ip.netns and ip.route to configure network stuff in the container.
func (t *T) NetNSPath() (string, error) {
	inspect, err := cli().ContainerService().Inspect(context.Background(), t.ContainerName())
	switch {
	case err == nil:
		return inspect.NetworkSettings.SandboxKey, nil
	case errdefs.IsNotFound(err):
		return "", nil
	default:
		return "", err
	}
}

// PID implements the resource.PIDer optional interface.
// Used by ip.netns to name the veth pair devices.
func (t *T) PID() int {
	cs := cli().ContainerService()
	name := t.ContainerName()
	inspect, err := cs.Inspect(context.Background(), name)
	if err != nil {
		return 0
	}
	return inspect.State.Pid
}

func (t *T) Status(ctx context.Context) status.T {
	if !t.Detach {
		return status.NotApplicable
	}
	if err := t.isDockerdPinging(ctx); err != nil {
		t.Log().Debug().Err(err).Msg("ping")
		t.StatusLog().Info("docker daemon is not running")
		return status.Down
	}
	inspect, err := cli().ContainerService().Inspect(ctx, t.ContainerName())
	switch {
	case err == nil:
	case errdefs.IsNotFound(err):
		return status.Down
	default:
		t.StatusLog().Error("inspect: %s", err)
		return status.Down
	}
	if t.Hostname != "" && inspect.Config.Hostname != t.Hostname {
		t.warnAttrDiff("hostname", inspect.Config.Hostname, t.Hostname)
	}
	if inspect.Config.OpenStdin != t.Interactive {
		t.warnAttrDiff("interactive", fmt.Sprint(inspect.Config.OpenStdin), fmt.Sprint(t.Interactive))
	}
	if len(t.Entrypoint) > 0 && !stringslice.Equal(inspect.Config.Entrypoint, t.Entrypoint) {
		t.warnAttrDiff("entrypoint", shellquote.Join(inspect.Config.Entrypoint...), shellquote.Join(t.Entrypoint...))
	}
	if inspect.Config.Tty != t.TTY {
		t.warnAttrDiff("tty", fmt.Sprint(inspect.Config.Tty), fmt.Sprint(t.TTY))
	}
	if inspect.HostConfig.Privileged != t.Privileged {
		t.warnAttrDiff("privileged", fmt.Sprint(inspect.HostConfig.Privileged), fmt.Sprint(t.Privileged))
	}
	t.statusInspectImage(ctx, inspect)
	t.statusInspectNS(ctx, "netns", inspect.HostConfig.NetworkMode, t.NetNS)
	t.statusInspectNS(ctx, "pidns", inspect.HostConfig.PidMode, t.PIDNS)
	t.statusInspectNS(ctx, "ipcns", inspect.HostConfig.IpcMode, t.IPCNS)
	t.statusInspectNS(ctx, "utsns", inspect.HostConfig.UTSMode, t.UTSNS)
	t.statusInspectNS(ctx, "userns", inspect.HostConfig.UsernsMode, t.UserNS)
	if !inspect.State.Running {
		return status.Down
	}
	return status.Up
}

func (t *T) statusInspectImage(ctx context.Context, inspect containerapi.ContainerInspect) {
	var tgtID, curID string
	if img, err := t.image(); err == nil {
		tgtID = img.ID()
	}
	if img, err := getImage(ctx, inspect.Config.Image); err == nil {
		curID = img.ID()
	}
	if curID != tgtID {
		t.warnAttrDiff("image", curID, tgtID)
	}
}

func (t *T) statusInspectNS(ctx context.Context, attr, current, target string) {
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
	r := t.GetObjectDriver().ResourceByID(rid.String())
	if r == nil {
		t.StatusLog().Warn("%s: %s resource not found", attr, target)
	} else if i, ok := r.(containerNamer); ok {
		name := i.ContainerName()
		tgt1 := "container:" + name
		tgt2 := "container:" + containerID(ctx, name)
		switch {
		case tgt1 == current:
			t.Log().Debug().Msgf("valid %s cross-resource reference to %s: %s", attr, tgt1, current)
		case tgt2 == current:
			t.Log().Debug().Msgf("valid %s cross-resource reference to %s: %s", attr, tgt2, current)
		default:
			t.warnAttrDiff(attr, current, tgt1)
		}
	}
}

func (t T) formatNS(s string) (string, error) {
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

func (t T) isDockerdPinging(ctx context.Context) error {
	_, err := cli().SystemService().Ping(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (t T) Label() string {
	return t.Image
}

func (t T) Provision(ctx context.Context) error {
	return nil
}

func (t T) Unprovision(ctx context.Context) error {
	return nil
}

func (t T) Provisioned() (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

func containerID(ctx context.Context, name string) string {
	inspect, err := cli().ContainerService().Inspect(ctx, name)
	if err != nil {
		return ""
	}
	return inspect.ID
}

// ContainerName formats a docker container name
func (t T) ContainerName() string {
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

func (t T) containerLabelID() string {
	return fmt.Sprintf("%s.%s", t.ObjectID, t.ResourceID.String())
}

func (t T) entrypoint(inspect imageapi.ImageInspect) []string {
	if len(t.Entrypoint) > 0 {
		return t.Entrypoint
	}
	return inspect.Config.Entrypoint
}

func (t T) command(inspect imageapi.ImageInspect) []string {
	if len(t.Command) > 0 {
		return t.Command
	}
	return inspect.Config.Cmd
}

func (t T) imageInspect() (imageapi.ImageInspect, error) {
	img, err := t.image()
	if err != nil {
		return imageapi.ImageInspect{}, err
	}
	return img.Inspect(context.Background())
}

func (t T) image() (*image.Image, error) {
	return getImage(context.Background(), t.Image)
}

func getImage(ctx context.Context, name string) (*image.Image, error) {
	if img, ok := imageCache.Get(name); ok {
		return img, nil
	}
	img, err := cli().ImageService().FindImage(ctx, name)
	if err != nil {
		return nil, err
	}
	imageCache.Put(name, img)
	return img, nil
}

func (t T) env() (env []string, err error) {
	return t.obfuscatableEnv(false)
}

func (t T) obfuscatedEnv() (env []string, err error) {
	return t.obfuscatableEnv(true)
}

func (t T) obfuscatableEnv(obfuscate bool) (env []string, err error) {
	var tempEnv []string
	env = []string{
		"OPENSVC_RID=" + t.RID(),
		"OPENSVC_NAME=" + t.Path.String(),
		"OPENSVC_KIND=" + t.Path.Kind.String(),
		"OPENSVC_ID=" + t.ObjectID.String(),
		"OPENSVC_NAMESPACE=" + t.Path.Namespace,
	}
	if len(t.Env) > 0 {
		env = append(env, t.Env...)
	}
	if tempEnv, err = envprovider.From(t.ConfigsEnv, t.Path.Namespace, "cfg"); err != nil {
		return nil, err
	}
	env = append(env, tempEnv...)
	if tempEnv, err = envprovider.From(t.SecretsEnv, t.Path.Namespace, "sec"); err != nil {
		return nil, err
	}
	if obfuscate {
		for i, s := range tempEnv {
			l := strings.SplitN(s, "=", 2)
			if len(l) != 2 {
				continue
			}
			tempEnv[i] = l[0] + "=xxx"
		}
	}
	env = append(env, tempEnv...)
	return env, nil
}

func (t T) Signal(sig syscall.Signal) error {
	cs := cli().ContainerService()
	name := t.ContainerName()
	inspect, err := cs.Inspect(context.Background(), name)
	switch {
	case err == nil:
	case errdefs.IsNotFound(err):
		t.Log().Info().Msgf("skip signal: container not found")
		return nil
	default:
		return err
	}
	if !inspect.State.Running {
		t.Log().Info().Msgf("skip signal: container not running")
		return nil
	}
	t.Log().Info().Int("pid", inspect.State.Pid).Str("signal", unix.SignalName(sig)).Msg("signal container")
	return syscall.Kill(inspect.State.Pid, sig)
}

func (t T) Enter() error {
	sh := "/bin/bash"
	name := t.ContainerName()
	cmd := exec.Command("docker", "exec", name, "/bin/bash")
	_ = cmd.Run()

	switch cmd.ProcessState.ExitCode() {
	case 126, 127:
		sh = "/bin/sh"
	}
	cmd = exec.Command("docker", "exec", "-it", name, sh)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (t T) LinkNames() []string {
	return []string{t.RID()}
}

func (t T) needDNS() bool {
	switch t.NetNS {
	case "", "none":
		return true
	default:
		return false
	}
}

func (t T) dns() []string {
	if !t.needDNS() {
		return []string{}
	}
	return t.DNS
}

func (t T) dnsOptions() []string {
	if !t.needDNS() {
		return []string{}
	}
	return []string{"ndots:2", "edns0", "use-vc"}
}

func (t T) dnsSearch() []string {
	if len(t.DNSSearch) > 0 {
		return t.DNSSearch
	}
	if !t.needDNS() {
		return []string{}
	}
	dom0 := fqdn.New(t.Path, rawconfig.ClusterSection().Name).Domain()
	dom1 := strings.SplitN(dom0, ".", 2)[1]
	dom2 := strings.SplitN(dom1, ".", 2)[1]
	return []string{dom0, dom1, dom2}
}

func (t T) needRemove() bool {
	return t.Remove || !t.Detach
}

func (t T) hostname() string {
	if !t.needDNS() {
		return ""
	}
	return t.Hostname
}

func (t T) stopTimeout() *int {
	if t.StopTimeout == nil {
		return nil
	}
	i := int(t.StopTimeout.Seconds())
	return &i
}
