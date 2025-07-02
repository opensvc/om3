package rescontainerlxc

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-ping/ping"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/vishvananda/netlink"
	"golang.org/x/crypto/ssh"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/actionrollback"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/core/vpath"
	"github.com/opensvc/om3/util/capabilities"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/envprovider"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/hostname"
)

const (
	cpusetDir = "/sys/fs/cgroup/cpuset"
)

var (
	prefixes = []string{
		"/",
		"/usr",
		"/usr/local",
	}
)

var _ resource.Encaper = (*T)(nil)

type (
	T struct {
		resource.T
		resource.SSH
		resource.SCSIPersistentReservation
		Path                     naming.Path    `json:"path"`
		ObjectID                 uuid.UUID      `json:"object_id"`
		Nodes                    []string       `json:"nodes"`
		DNS                      []string       `json:"dns"`
		SCSIReserv               bool           `json:"scsireserv"`
		PromoteRW                bool           `json:"promote_rw"`
		OsvcRootPath             string         `json:"osvc_root_path"`
		GuestOS                  string         `json:"guest_os"`
		Name                     string         `json:"name"`
		Hostname                 string         `json:"hostname"`
		DataDir                  string         `json:"data_dir"`
		RootDir                  string         `json:"root_dir"`
		ConfigFile               string         `json:"cf"`
		Template                 string         `json:"template"`
		TemplateOptions          []string       `json:"template_options"`
		CreateSecretsEnvironment []string       `json:"create_secrets_environment"`
		CreateConfigsEnvironment []string       `json:"create_configs_environment"`
		CreateEnvironment        []string       `json:"create_environment"`
		RCmd                     []string       `json:"rcmd"`
		StartTimeout             *time.Duration `json:"start_timeout"`
		StopTimeout              *time.Duration `json:"stop_timeout"`

		cache map[string]interface{}
	}

	header interface {
		Head() string
	}
	resourceLister interface {
		Resources() resource.Drivers
	}
)

func New() resource.Driver {
	t := &T{
		cache: make(map[string]interface{}),
	}
	return t
}

func (t *T) stopCgroup() error {
	p := t.cgroupDir()
	if p == "" {
		return nil
	}
	p = filepath.Join(cpusetDir, p)
	if err := t.cleanupCgroup(p); err != nil {
		return err
	}
	return nil
}

func (t *T) startCgroup() error {
	p := t.cgroupDir()
	if p == "" {
		return nil
	}
	p = filepath.Join(cpusetDir, p)
	if err := t.cleanupCgroup(p); err != nil {
		return err
	}
	if err := t.setCpusetCloneChildren(); err != nil {
		return err
	}
	if err := t.createCgroup(p); err != nil {
		return err
	}
	return nil
}

func (t *T) Start(ctx context.Context) error {
	if v, err := t.isUp(); err != nil {
		return err
	} else if v {
		t.Log().Infof("container %s is already up", t.Name)
		return nil
	}
	if err := t.startCgroup(); err != nil {
		return err
	}
	if err := t.installCF(); err != nil {
		return err
	}
	if err := t.start(ctx); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.Stop(ctx)
	})
	return nil
}

func (t *T) Stop(ctx context.Context) error {
	if v, err := t.isUp(); err != nil {
		return err
	} else if !v {
		t.Log().Infof("container %s is already down", t.Name)
		return nil
	}
	links := t.getLinks()
	if err := t.stopOrKill(ctx); err != nil {
		return err
	}
	if err := t.cleanupLinks(links); err != nil {
		return err
	}
	if err := t.stopCgroup(); err != nil {
		return err
	}
	return nil
}

// NetNSPath implements the resource.NetNSPather optional interface.
// Used by ip.netns and ip.route to configure network stuff in the container.
func (t *T) NetNSPath(ctx context.Context) (string, error) {
	if pid, err := t.getPID(ctx); err != nil {
		return "", err
	} else if pid == 0 {
		return "", fmt.Errorf("container %s is not running", t.Name)
	} else {
		return fmt.Sprintf("/proc/%d/ns/net", pid), nil
	}
}

// PID implements the resource.PIDer optional interface.
// Used by ip.netns to name the veth pair devices.
func (t *T) PID(ctx context.Context) int {
	pid, _ := t.getPID(ctx)
	return pid
}

func (t *T) Status(ctx context.Context) status.T {
	/*
		if t.PG.IsFrozen() {
			return status.NotApplicable
		}
	*/
	if v, err := t.isUp(); err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	} else if v {
		return status.Up
	}
	return status.Down
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	return t.Name
}

func (t *T) UnprovisionAsLeader(ctx context.Context) error {
	return t.unprovision()
}

func (t *T) UnprovisionAsFollower(ctx context.Context) error {
	return t.unprovision()
}

func (t *T) unprovision() error {
	if err := t.purgeLxcVar(); err != nil {
		return err
	}
	if err := t.purgeConfigFile(); err != nil {
		return err
	}
	return nil
}

func (t *T) purgeConfigFile() error {
	p, err := t.configFile()
	if err != nil {
		return err
	}
	if !file.Exists(p) {
		return nil
	}
	t.Log().Infof("remove %s", p)
	if err := os.Remove(p); err != nil {
		return err
	}
	return nil
}

func (t *T) purgeLxcVar() error {
	p := t.lxcPath()
	if p == "" {
		t.Log().Debugf("purgeLxcVar: lxcPath() is empty. consider we have nothing to purge.")
		return nil
	}
	p = filepath.Join(p, t.Name)
	if !file.Exists(p) {
		t.Log().Infof("%s is already cleaned up", p)
		return nil
	}
	if file.IsProtected(p) {
		t.Log().Warnf("refuse to remove %s", p)
		return nil
	}
	t.Log().Infof("remove %s", p)
	if err := os.RemoveAll(p); err != nil {
		return err
	}
	return nil
}

func (t *T) ProvisionAsLeader(ctx context.Context) error {
	if t.exists() {
		t.Log().Infof("container %s is already created", t.Name)
		return nil
	}
	args := []string{"--name", t.Name}
	rootDir, err := t.rootDir()
	if err == nil {
		if !file.Exists(rootDir) {
			if err := os.MkdirAll(rootDir, 0755); err != nil {
				return err
			}
		}
		args = append(args, "--dir", rootDir)
	}
	cf, err := t.configFile()
	if err == nil && cf != "" && file.Exists(cf) {
		args = append(args, "-f", cf)
	}
	dataDir, err := t.dataDir()
	if err == nil && dataDir != "" {
		args = append(args, "-P", dataDir)
		if cf == "" {
			cf = filepath.Join(dataDir, t.Name, "config")
			if file.Exists(cf) {
				args = append(args, "-f", cf)
			}
		}
	}
	if t.Template != "" {
		args = append(args, "-t", t.Template)
		if len(t.TemplateOptions) > 0 {
			args = append(args, "..")
			args = append(args, t.TemplateOptions...)
		}
	} else {
		return fmt.Errorf("the template keyword is mandatory for provision")
	}
	env, err := t.createEnv()
	if err != nil {
		return err
	}
	cmd := command.New(
		command.WithName("lxc-create"),
		command.WithArgs(args),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		//command.WithTimeout(*t.StartTimeout),
		command.WithEnv(env),
	)
	return cmd.Run()
}

func (t *T) createEnv() ([]string, error) {
	env := []string{
		"DEBIAN_FRONTEND=noninteractive",
		"DEBIAN_PRIORITY=critical",
	}
	env = append(env, t.createEnvProxy()...)
	env = append(env, t.CreateEnvironment...)
	more, err := envprovider.From(t.CreateSecretsEnvironment, t.Path.Namespace, "sec")
	if err != nil {
		return env, err
	}
	env = append(env, more...)
	more, err = envprovider.From(t.CreateConfigsEnvironment, t.Path.Namespace, "cfg")
	if err != nil {
		return env, err
	}
	env = append(env, more...)
	return env, nil
}

func (t *T) createEnvSecrets() ([]string, error) {
	return envprovider.From(t.CreateSecretsEnvironment, t.Path.Namespace, "sec")
}

func (t *T) createEnvConfigs() ([]string, error) {
	return envprovider.From(t.CreateConfigsEnvironment, t.Path.Namespace, "cfg")
}

func (t *T) createEnvProxy() []string {
	env := []string{}
	keys := []string{
		"http_proxy", "https_proxy", "ftp_proxy", "rsync_proxy",
	}
	for _, k := range keys {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
		k = strings.ToUpper(k)
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}

func (t *T) Unprovision(ctx context.Context) error {
	return nil
}

func (t *T) Provisioned() (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

// Signal implements object.signaler
func (t *T) Signal(ctx context.Context, sig syscall.Signal) error {
	pid := t.PID(ctx)
	if pid == 0 {
		return nil
	}
	return syscall.Kill(pid, sig)
}

func (t *T) copyFrom(src, dst string) error {
	rootDir, err := t.rootDir()
	if err != nil {
		return err
	}
	src = filepath.Join(rootDir, src)
	return file.Copy(src, dst)
}

func (t *T) copyTo(src, dst string) error {
	rootDir, err := t.rootDir()
	if err != nil {
		return err
	}
	dst = filepath.Join(rootDir, dst)
	return file.Copy(src, dst)
}

func (t *T) rcmd(envs []string) ([]string, error) {
	var args []string
	if len(t.RCmd) > 0 {
		args = t.RCmd
	} else {
		hasPIDNS := file.Exists("/proc/1/ns/pid")
		if exe, err := exec.LookPath("lxc-attach"); err == nil && hasPIDNS {
			if p, err := t.dataDir(); err == nil && p != "" {
				args = []string{exe, "-n", t.Name, "-P", p, "--clear-env"}
			} else {
				args = []string{exe, "-n", t.Name, "--clear-env"}
			}
		}
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("unable to identify a remote command method. install lxc-attach or set the rcmd keyword")
	}
	for _, e := range envs {
		args = append(args, "-v", e)
	}
	if args[len(args)-1] != "--" {
		args = append(args, "--")
	}
	return args, nil
}

// SetEncapFileOwnership sets the ownership of the file to be the
// same ownership than the container root dir, which may be not root
// for unprivileged containers.
func (t *T) SetEncapFileOwnership(p string) error {
	rootDir, err := t.rootDir()
	if err != nil {
		return err
	}
	return file.CopyOwnership(rootDir, p)
}

func (t *T) Enter() error {
	sh := "/bin/bash"
	rcmd, err := t.rcmd([]string{})
	if err != nil {
		return err
	}
	args := append(rcmd, sh)
	cmd := exec.Command(args[0], args[1:]...)
	_ = cmd.Run()

	switch cmd.ProcessState.ExitCode() {
	case 126, 127:
		sh = "/bin/sh"
	}
	args = append(rcmd, sh)
	cmd = exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (t *T) stopTimeout() *int {
	if t.StopTimeout == nil {
		return nil
	}
	i := int(t.StopTimeout.Seconds())
	return &i
}

func (t *T) GetHostname() string {
	if t.Hostname != "" {
		return t.Hostname
	}
	return t.Name
}

func (t *T) setHostname() error {
	if err := t.checkHostname(); err != nil {
		t.Log().Infof("container hostname already set")
		return nil
	}
	p, err := t.hostnameFile()
	if err != nil {
		return err
	}
	h := t.GetHostname()
	if err := os.WriteFile(p, []byte(h+"\n"), 0644); err != nil {
		return err
	}
	t.Log().Infof("container hostname set to %s", h)
	return nil
}

func (t *T) checkHostname() error {
	p, err := t.hostnameFile()
	if err != nil {
		return err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return fmt.Errorf("can not read container hostname: %w", err)
	}
	target := t.GetHostname()
	if string(b) != target {
		return fmt.Errorf("container hostname is %s, should be %s", string(b), target)
	}
	return nil
}

func (t *T) hostnameFile() (string, error) {
	rootDir, err := t.rootDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(rootDir, "etc/hostname"), nil
}

func (t *T) configFile() (string, error) {
	if p, ok := t.cache["configFile"]; ok {
		return p.(string), nil
	}
	if p, err := t.getConfigFile(); err == nil {
		t.cache["configFile"] = p
		return p, nil
	} else {
		return "", err
	}
}

func (t *T) getPrefix() (string, error) {
	for _, p := range prefixes {
		f := filepath.Join(p, "bin/lxc-start")
		if file.Exists(f) {
			return p, nil
		}
	}
	return "", fmt.Errorf("lxc install prefix not found")
}

// orderedPrefixes returns prefixes with the one containing lxc-start first
func (t *T) orderedPrefixes(prefix string) ([]string, error) {
	l := []string{prefix}
	for _, p := range prefixes {
		if p == prefix {
			continue
		}
		l = append(l, p)
	}
	return l, nil
}

func (t *T) prefix() (string, error) {
	if p, ok := t.cache["prefix"]; ok {
		return p.(string), nil
	}
	if p, err := t.getPrefix(); err == nil {
		t.cache["prefix"] = p
		return p, nil
	} else {
		return "", err
	}
}

func (t *T) getConfigFile() (string, error) {
	if t.ConfigFile != "" {
		return vpath.HostPath(t.ConfigFile, t.Path.Namespace)
	}
	if t.DataDir != "" {
		p := filepath.Join(t.DataDir, t.Name, "config")
		return vpath.HostPath(p, t.Path.Namespace)
	}
	relDir := "/var/lib/lxc"

	// seen on debian squeeze : prefix is /usr, but containers'
	// config files paths are /var/lib/lxc/$name/config
	// try prefix first, fallback to other know prefixes
	prefix, err := t.prefix()
	if err != nil {
		return "", err
	}
	prefixes, err := t.orderedPrefixes(prefix)
	if err != nil {
		return "", err
	}
	for _, p := range prefixes {
		p = filepath.Join(p, relDir, t.Name, "config")
		if file.Exists(p) {
			return p, nil
		}
	}

	// on Oracle Linux, config is in /etc/lxc
	p := filepath.Join("/etc/lxc", t.Name, "config")
	if file.Exists(p) {
		return p, nil
	}

	return "", fmt.Errorf("unable to find the container configuration file")
}

func (t *T) getConfigValue(key string) (string, error) {
	cf, err := t.configFile()
	f, err := os.Open(cf)
	if err != nil {
		return "", err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		s := scanner.Text()
		l := strings.SplitN(s, "=", 2)
		if len(l) != 2 {
			continue
		}
		k := l[0]
		k = strings.TrimLeft(k, "#; \t")
		k = strings.TrimSpace(k)
		if key != k {
			continue
		}
		v := l[1]
		v = strings.TrimSpace(v)
		return v, nil
	}
	return "", fmt.Errorf("key %s not found in %s", key, cf)
}

func (t *T) rootDirFromConfigFile() (string, error) {
	p, err := t.rootfsFromConfigFile()
	if err != nil {
		return "", err
	}
	if !strings.Contains(p, ":") {
		return p, nil
	}
	// zfs:/tank/svc1, nbd:file1, dir:/foo ...
	l := strings.SplitN(p, ":", 2)
	p = l[1]
	if p != "" && !strings.HasPrefix(p, "/") {
		// zfs:tank/svc1
		p = "/" + p
	}
	return p, nil
}

func (t *T) rootfsFromConfigFile() (string, error) {
	if p, err := t.getConfigValue("lxc.rootfs"); err == nil {
		return p, nil
	}
	if p, err := t.getConfigValue("lxc.rootfs.path"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("could not determine lxc container rootfs")
}

func (t *T) getRootDir() (string, error) {
	if t.RootDir != "" {
		return vpath.HostPath(t.RootDir, t.Path.Namespace)
	}
	return t.rootDirFromConfigFile()
}

func (t *T) rootDir() (string, error) {
	if p, ok := t.cache["rootDir"]; ok {
		return p.(string), nil
	}
	if p, err := t.getRootDir(); err == nil {
		t.cache["rootDir"] = p
		return p, nil
	} else {
		return "", err
	}
}

func (t *T) dataDir() (string, error) {
	if t.DataDir == "" {
		return "", nil
	}
	if p, ok := t.cache["dataDir"]; ok {
		return p.(string), nil
	}
	if p, err := vpath.HostPath(t.DataDir, t.Path.Namespace); err == nil {
		t.cache["dataDir"] = p
		return p, nil
	} else {
		return "", err
	}
}

func (t *T) nativeConfigFile() string {
	if p, ok := t.cache["nativeConfigFile"]; ok {
		return p.(string)
	}
	p := func() string {
		if p := t.lxcPath(); p != "" {
			return filepath.Join(p, t.Name, "config")
		}
		exe, err := exec.LookPath("lxc-info")
		if err != nil {
			return ""
		}
		dir := filepath.Dir(exe)
		if !strings.HasSuffix(dir, "bin") {
			return ""
		}
		dir = filepath.Dir(dir)
		switch dir {
		case "/", "/usr":
			if v, _ := file.ExistsAndDir("/var/lib/lxc"); v {
				return fmt.Sprintf("/var/lib/lxc/%s/config", t.Name)
			}
			if v, _ := file.ExistsAndDir("/etc/lxc"); v {
				return fmt.Sprintf("/etc/lxc/%s/config", t.Name)
			}
		case "/usr/local":
			if v, _ := file.ExistsAndDir("/usr/local/var/lib/lxc"); v {
				return fmt.Sprintf("/usr/local/var/lib/lxc/%s/config", t.Name)
			}
		}
		return ""
	}()
	t.cache["nativeConfigFile"] = p
	return p
}

func (t *T) lxcPath() string {
	if p, ok := t.cache["lxcPath"]; ok {
		return p.(string)
	}
	p := func() string {
		if p, err := t.dataDir(); err != nil {
			return p
		}
		p := "/var/lib/lxc"
		if v, _ := file.ExistsAndDir(p); v {
			return p
		}
		p = "/usr/local/var/lib/lxc"
		if v, _ := file.ExistsAndDir(p); v {
			return p
		}
		return ""
	}()
	t.cache["lxcPath"] = p
	return p
}

func (t *T) ToSync() []string {
	l := make([]string, 0)

	// Don't synchronize container.lxc config in /var/lib/lxc if not shared
	// Non shared container resource mandates a private container for each
	// service instance, thus synchronizing the lxc config is counter productive
	// and can even lead to provisioning failure on secondary nodes.
	if !t.Shared {
		return l
	}

	// The config file might be in a umounted fs resource,
	// in which case, no need to ask for its sync as the sync won't happen
	cf, err := t.configFile()
	if err != nil {
		return l
	}
	if !file.Exists(cf) {
		return l
	}

	// The config file is hosted on a fs resource.
	// Let the user replicate it via a sync resource if the fs is not
	// shared. If the fs is shared, it must not be replicated to avoid
	// copying on the remote underlying fs (which may block a zfs dataset
	// mount).
	r, err := t.resourceHandlingFile(cf)
	if err != nil {
		return l
	}
	if r == nil {
		return l
	}

	// replicate the config file in the system standard path
	l = append(l, cf)
	return l

}

func (t *T) obj() (interface{}, error) {
	return object.New(t.Path, object.WithVolatile(true))
}

func (t *T) resourceHandlingFile(p string) (resource.Driver, error) {
	obj, err := t.obj()
	if err != nil {
		return nil, err
	}
	b, ok := obj.(resourceLister)
	if !ok {
		return nil, nil
	}
	for _, r := range b.Resources() {
		h, ok := r.(header)
		if !ok {
			continue
		}
		if v, err := r.Provisioned(); err != nil {
			continue
		} else if v == provisioned.False {
			continue
		}
		if h.Head() == p {
			return r, nil
		}
	}
	return nil, nil
}

// ContainerHead implements the interface replacing b2.1 the zonepath resource attribute
func (t *T) ContainerHead() (string, error) {
	return t.rootDir()
}

func (t *T) cpusetDir() string {
	path := ""
	if !file.Exists(cpusetDir) {
		t.Log().Debugf("startCgroup: %s does not exist", cpusetDir)
		return ""
	}
	if t.cgroupDirCapable() {
		p := t.cgroupDir()
		if p == "" {
			return ""
		}
		path = filepath.Join(cpusetDir, p)
	} else {
		path = filepath.Join(cpusetDir, "lxc")
	}
	return path
}

func (t *T) setCpusetCloneChildren() error {
	path := t.cpusetDir()
	if path == "" {
		return nil
	}
	if !file.Exists(path) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("set clone_children for container %s: %w", t.Name, err)
		} else {
			t.Log().Infof("%s created", path)
		}
	}
	paths := make([]string, 0)
	for p := path; p != cpusetDir; p = filepath.Dir(p) {
		paths = append(paths, p)
	}
	sort.Sort(sort.StringSlice(paths))

	setFile := func(p string, v []byte) error {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Log().Debugf("%s does not exist", p)
			return nil
		}
		if bytes.Compare(b, v) == 0 {
			t.Log().Debugf("%s already set to %s", p, v)
			return nil
		}
		err = os.WriteFile(p, v, 0644)
		if err != nil {
			return err
		}
		t.Log().Infof("%s set to %s", p, v)
		return nil
	}
	alignFile := func(p string) error {
		base := filepath.Base(p)
		ref := filepath.Join(cpusetDir, base)
		b, err := os.ReadFile(ref)
		if err != nil {
			t.Log().Debugf("%s does not exist", ref)
			return nil
		}
		return setFile(p, b)
	}
	setDir := func(p string) error {
		if err := alignFile(filepath.Join(p, "cpuset.mems")); err != nil {
			return err
		}
		if err := alignFile(filepath.Join(p, "cpuset.cpus")); err != nil {
			return err
		}
		if err := setFile(filepath.Join(p, "cgroup.clone_children"), []byte("1\n")); err != nil {
			return err
		}
		return nil
	}

	for _, p := range paths {
		if err := setDir(p); err != nil {
			return err
		}
	}
	return nil
}

// cgroupDir returns the container resource cgroup path, relative to a controller head.
func (t *T) cgroupDir() string {
	return strings.TrimPrefix(t.GetPGID(), "/")
}

func (t *T) cgroupDirCapable() bool {
	return capabilities.Has(drvID.Cap() + ".cgroup_dir")
}

func (t *T) createCgroup(p string) error {
	if file.Exists(p) {
		t.Log().Debugf("%s already exists", p)
		return nil
	}
	if err := os.MkdirAll(p, 0755); err != nil {
		return fmt.Errorf("create %s: %w", p, err)
	}
	t.Log().Infof("%s created", p)
	return nil
}

func (t *T) cleanupCgroup(p string) error {
	patterns := []string{
		fmt.Sprintf("/sys/fs/cgroup/*/lxc/%s-[0-9]", t.Name),
		fmt.Sprintf("/sys/fs/cgroup/*/lxc/%s", t.Name),
		fmt.Sprintf("%s", p),
		fmt.Sprintf("%s/lxc.*", p),
	}
	paths := []string{}
	for _, pattern := range patterns {
		more, err := filepath.Glob(pattern)
		if err != nil {
			return err
		}
		paths = append(paths, more...)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(paths)))
	for _, path := range paths {
		t.Log().Infof("remove %s", path)
		if err := os.Remove(path); err != nil {
			t.Log().Warnf("%s", err)
		}
	}
	return nil
}

func (t *T) installCF() error {
	cf, err := t.configFile()
	if err != nil {
		return err
	}
	nativeCF := t.nativeConfigFile()
	if nativeCF == "" {
		t.Log().Debugf("could not determine the config file standard hosting directory")
		return nil
	}
	if cf == nativeCF {
		return nil
	}
	nativeDir := filepath.Dir(nativeCF)
	if !file.Exists(nativeDir) {
		if err := os.MkdirAll(nativeDir, 0755); err != nil {
			return err
		}
	}
	if err := file.Copy(cf, nativeCF); err != nil {
		return fmt.Errorf("install %s as %s: %w", cf, nativeCF, err)
	}
	t.Log().Infof("%s installed as %s", cf, nativeCF)
	return err
}

func (t *T) dataDirArgs() []string {
	if dataDir, err := t.dataDir(); err == nil && dataDir != "" {
		return []string{"-P", dataDir}
	}
	return []string{}
}

func (t *T) isUpInfo() bool {
	args := []string{"--name", t.Name}
	args = append(args, t.dataDirArgs()...)
	cmd := command.New(
		command.WithName("lxc-info"),
		command.WithArgs(args),
		command.WithBufferedStdout(),
	)
	b, err := cmd.Output()
	if err != nil {
		return false
	}
	v := strings.Contains(string(b), "RUNNING")
	return v
}

func (t *T) exists() bool {
	args := []string{"--name", t.Name}
	args = append(args, t.dataDirArgs()...)
	cmd := command.New(
		command.WithName("lxc-info"),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		//command.WithCommandLogLevel(zerolog.InfoLevel),
		//command.WithStdoutLogLevel(zerolog.InfoLevel),
		//command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run() == nil
}

func (t *T) cleanupLink(s string) error {
	link, err := netlink.LinkByName(s)
	if err != nil {
		t.Log().Debugf("link %s already deleted", s)
		return nil
	}
	if err := netlink.LinkDel(link); err != nil {
		return fmt.Errorf("link %s delete: %w", s, err)
	}

	t.Log().Infof("link %s deleted", s)
	return nil
}

func (t *T) cleanupLinks(links []string) error {
	for _, link := range links {
		if err := t.cleanupLink(link); err != nil {
			return err
		}
	}
	return nil
}

func (t *T) getPID(ctx context.Context) (int, error) {
	args := []string{"--name", t.Name}
	args = append(args, t.dataDirArgs()...)
	opts := []funcopt.O{
		command.WithName("lxc-info"),
		command.WithArgs(args),
		command.WithBufferedStdout(),
	}
	if ctx != nil {
		opts = append(opts, command.WithContext(ctx))
	}
	cmd := command.New(opts...)
	b, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(b))
	for scanner.Scan() {
		s := scanner.Text()
		if strings.HasPrefix(s, "PID:") {
			fields := strings.Fields(s)
			if len(fields) < 2 {
				continue
			}
			return strconv.Atoi(fields[1])
		}
	}
	return 0, fmt.Errorf("pid not found")
}

func (t *T) getLinks() []string {
	l := make([]string, 0)
	args := []string{"--name", t.Name}
	args = append(args, t.dataDirArgs()...)
	cmd := command.New(
		command.WithName("lxc-info"),
		command.WithArgs(args),
		command.WithBufferedStdout(),
	)
	b, err := cmd.Output()
	if err != nil {
		return l
	}
	scanner := bufio.NewScanner(bytes.NewReader(b))
	for scanner.Scan() {
		s := scanner.Text()
		if strings.HasPrefix(s, "Link:") {
			fields := strings.Fields(s)
			if len(fields) < 2 {
				continue
			}
			l = append(l, fields[1])
		}
	}
	return l
}

func (t *T) isUpPS() bool {
	cmd := command.New(
		command.WithName("lxc-ps"),
		command.WithVarArgs("--name", t.Name),
		command.WithBufferedStdout(),
	)
	b, err := cmd.Output()
	if err != nil {
		return false
	}
	v := strings.Contains(string(b), t.Name)
	return v
}

func (t *T) isUp() (bool, error) {
	if p, err := exec.LookPath("lxc-ps"); err == nil && p != "" {
		return t.isUpPS(), nil
	}
	return t.isUpInfo(), nil
}

func (t *T) start(ctx context.Context) error {
	cgroupDir := t.cgroupDir()
	cf, err := t.configFile()
	if err != nil {
		return err
	}
	outFile := fmt.Sprintf("/var/tmp/svc_%s_lxc_start.log", t.Name)
	args := []string{"-d", "-n", t.Name, "-o", outFile}
	if t.cgroupDirCapable() {
		args = append(args, "-s", "lxc.cgroup.dir="+cgroupDir)
	}
	if cf != "" {
		args = append(args, "-f", cf)
	}
	args = append(args, t.dataDirArgs()...)
	cmd := command.New(
		command.WithName("lxc-start"),
		command.WithArgs(args),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithTimeout(*t.StartTimeout),
	)
	return cmd.Run()
}

func (t *T) stopOrKill(ctx context.Context) error {
	if actioncontext.IsForce(ctx) {
		return t.kill()
	}
	if err := t.stop(); err == nil {
		return err
	} else {
		t.Log().Warnf("stop: %s", err)
	}
	return t.kill()
}

func (t *T) stop() error {
	args := []string{"-n", t.Name}
	args = append(args, t.dataDirArgs()...)
	cmd := command.New(
		command.WithName("lxc-stop"),
		command.WithArgs(args),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithTimeout(*t.StartTimeout),
	)
	return cmd.Run()
}

func (t *T) kill() error {
	args := []string{"-n", t.Name, "--kill"}
	args = append(args, t.dataDirArgs()...)
	cmd := command.New(
		command.WithName("lxc-stop"),
		command.WithArgs(args),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithTimeout(*t.StartTimeout),
	)
	return cmd.Run()
}

// LinkNames implements the interface necessary for the container.lxc resources
// to be targeted by ip.cni, ip.netns, ...
func (t *T) LinkNames() []string {
	return []string{t.RID()}
}

func (t *T) Abort(ctx context.Context) bool {
	if v, err := t.isUp(); err != nil {
		t.Log().Warnf("abort? %s", err)
		return false
	} else if v {
		// the local instance is already up.
		// let the local start report the unnecessary start steps
		return false
	}
	hn := t.GetHostname()
	t.Log().Infof("abort? ping %s", hn)

	if pinger, err := ping.NewPinger(t.GetHostname()); err == nil {
		pinger.Timeout = time.Second * 5
		pinger.Count = 1
		if err := pinger.Run(); err != nil {
			t.Log().Warnf("abort? pinger err: %s", err)
			return false
		}
		if pinger.Statistics().PacketsRecv > 0 {
			t.Log().Infof("abort! %s is alive", hn)
			return true
		}
		t.Log().Debugf("abort? %s is not alive", hn)
		return false
	} else {
		t.Log().Debugf("abort? pinger init failed: %s", err)
	}
	if n, err := t.upPeer(); err != nil {
		return false
	} else if n != "" {
		t.Log().Infof("abort! %s is up on %s", hn, n)
		return true
	}
	return false
}

func (t *T) upPeer() (string, error) {
	hn := hostname.Hostname()
	isPeerUp := func(n string) (bool, error) {
		client, err := t.NewSSHClient(n)
		if err != nil {
			return false, err
		}
		defer client.Close()
		session, err := client.NewSession()
		if err != nil {
			return false, err
		}
		defer session.Close()
		var b bytes.Buffer
		session.Stdout = &b
		err = session.Run(fmt.Sprintf("lxc-info -n %s -p", t.Name))
		if err == nil {
			return true, nil
		}
		ee := err.(*ssh.ExitError)
		ec := ee.Waitmsg.ExitStatus()
		return ec == 0, err
	}
	for _, n := range t.Nodes {
		if hn == n {
			continue
		}
		if v, err := isPeerUp(n); err != nil {
			t.Log().Debugf("ssh abort check on %s: %s", n, err)
			continue
		} else if v {
			return n, nil
		}
	}
	return "", nil
}

func (t *T) EncapCmd(ctx context.Context, args []string, envs []string) (resource.Commander, error) {
	baseArgs, err := t.rcmd(envs)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(baseArgs[0], append(baseArgs[1:], args...)...)
	return cmd, nil
}

func (t *T) EncapCp(ctx context.Context, src, dst string) error {
	rootDir, err := t.rootDir()
	if err != nil {
		return err
	}
	dst = filepath.Join(rootDir, dst)
	return file.Copy2(src, dst)
}

func (t *T) GetOsvcRootPath() string {
	if t.OsvcRootPath != "" {
		return filepath.Join(t.OsvcRootPath, "bin", "om")
	}
	return filepath.Join(rawconfig.Paths.Bin, "om")
}
