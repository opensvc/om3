package rescontainerkvm

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/antchfx/xmlquery"
	"github.com/go-ping/ping"
	"github.com/google/uuid"
	"github.com/hashicorp/go-version"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/opensvc/om3/core/actionrollback"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/core/topology"
	"github.com/opensvc/om3/util/capabilities"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/device"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/waitfor"
)

const (
	cpusetDir = "/sys/fs/cgroup/cpuset"

	DomStateBlocked = "blocked"
	DomStateCrashed = "crashed"
	DomStateNone    = "no state"
	DomStatePaused  = "paused"
	DomStateRunning = "running"
	DomStateShutOff = "shut off"
)

type (
	T struct {
		resource.T
		resource.SSH
		resource.SCSIPersistentReservation
		Path       naming.Path `json:"path"`
		ObjectID   uuid.UUID   `json:"object_id"`
		Peers      []string    `json:"peers"`
		EncapNodes []string    `json:"encapnodes"`
		DNS        []string    `json:"dns"`
		Topology   topology.T

		SCSIReserv   bool           `json:"scsireserv"`
		PromoteRW    bool           `json:"promote_rw"`
		OsvcRootPath string         `json:"osvc_root_path"`
		GuestOS      string         `json:"guest_os"`
		Name         string         `json:"name"`
		Hostname     string         `json:"hostname"`
		RCmd         []string       `json:"rcmd"`
		StartTimeout *time.Duration `json:"start_timeout"`
		StopTimeout  *time.Duration `json:"stop_timeout"`
		VirtInst     []string       `json:"virtinst"`
		QGA          bool           `json:"qga"`
		//Snap           string         `json:"snap"`
		//SnapOf         string         `json:"snapof"`

		cache map[string]interface{}
	}

	header interface {
		Head() string
	}
	resourceLister interface {
		Resources() resource.Drivers
	}
)

var _ resource.Encaper = (*T)(nil)

func isPartitionsCapable() bool {
	cmd := command.New(
		command.WithName("virsh"),
		command.WithVarArgs("--version"),
		command.WithBufferedStdout(),
	)
	b, err := cmd.Output()
	if err != nil {
		return false
	}
	vs := strings.TrimSpace(string(b))
	v, err := version.NewVersion(vs)
	if err != nil {
		return false
	}
	constraints, err := version.NewConstraint(">= 1.0.1")
	if err != nil {
		return false
	}
	if constraints.Check(v) {
		return true
	}
	return false
}

func isHVMCapable() bool {
	cmd := command.New(
		command.WithName("virsh"),
		command.WithVarArgs("capabilities"),
		command.WithBufferedStdout(),
	)
	b, err := cmd.Output()
	if err != nil {
		return false
	}
	if bytes.Contains(b, []byte("hvm")) {
		return true
	}
	return false
}

func New() resource.Driver {
	t := &T{
		cache: make(map[string]interface{}),
	}
	return t
}

func (t *T) configFile() string {
	return filepath.Join("/etc/libvirt/qemu", t.Name+".xml")
}

func (t *T) autostartFile() string {
	return filepath.Join("/etc/libvirt/qemu/autostart/", t.Name+".xml")
}

func (t *T) configFiles() []string {
	files := make([]string, 0)
	if !t.IsShared() && t.Topology != topology.Failover {
		// don't send the container cf to nodes that won't run it
		return files
	}
	cf := t.configFile()
	if !file.Exists(cf) {
		return files
	}
	files = append(files, cf)
	if firmwareFiles, err := t.firmwareFiles(); err != nil {
		t.Log().Warnf("list firmware files: %s", err)
	} else {
		files = append(files, firmwareFiles...)
	}
	return files
}

func (t *T) ToSync() []string {
	return t.configFiles()
}

func (t *T) checkCapabilities() bool {
	if !capabilities.Has(drvID.Cap() + ".hvm") {
		t.StatusLog().Warn("hvm not supported by host")
		return false
	}
	return true
}

func (t *T) hasEncap() bool {
	return slices.Contains(t.EncapNodes, t.Name)
}

func (t *T) isOperational() (bool, error) {
	if err := t.rexec("pwd"); err != nil {
		t.Log().Debugf("isOperational: %s", err)
		return false, nil
	}
	return true, nil
}

func (t *T) isPinging() (bool, error) {
	pinger, err := ping.NewPinger(t.GetHostname())
	if err != nil {
		return false, err
	}
	pinger.Timeout = time.Second * 1
	pinger.Count = 1
	if err := pinger.Run(); err != nil {
		return false, err
	}
	if pinger.Statistics().PacketsRecv > 0 {
		return true, nil
	}
	return false, nil
}

func (t *T) define() error {
	cmd := command.New(
		command.WithName("virsh"),
		command.WithVarArgs("define", t.configFile()),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}

func (t *T) undefine() error {
	args := []string{"undefine", t.Name}
	if hasEFI, err := t.HasEFI(); err != nil {
		return err
	} else if hasEFI {
		args = append(args, "--nvram")
	}
	cmd := command.New(
		command.WithName("virsh"),
		command.WithArgs(args),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}

func (t *T) start() error {
	cmd := command.New(
		command.WithName("virsh"),
		command.WithVarArgs("start", t.Name),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithTimeout(*t.StartTimeout),
	)
	return cmd.Run()
}

func (t *T) stop() error {
	cmd := command.New(
		command.WithName("virsh"),
		command.WithVarArgs("shutdown", t.Name),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithTimeout(*t.StopTimeout),
	)
	return cmd.Run()
}

func (t *T) destroy() error {
	cmd := command.New(
		command.WithName("virsh"),
		command.WithVarArgs("destroy", t.Name),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithTimeout(*t.StopTimeout),
	)
	return cmd.Run()
}

func (t *T) containerStart(ctx context.Context) error {
	if !t.hasConfigFile() {
		return fmt.Errorf("%s not found", t.configFile())
	}
	if err := t.doPartitions(); err != nil {
		return err
	}
	if err := t.define(); err != nil {
		return err
	}
	if err := t.start(); err != nil {
		return err
	}
	return nil
}

func (t *T) doPartitions() error {
	if t.GetPG() != nil && !capabilities.Has("node.x.machinectl") && capabilities.Has(drvID.Cap()+".partitions") {
		if err := t.setPartitions(); err != nil {
			return err
		}
	} else {
		if err := t.unsetPartitions(); err != nil {
			return err
		}
	}
	return nil
}

func (t *T) Start(ctx context.Context) error {
	if err := t.ApplyPGChain(ctx); err != nil {
		return err
	}
	if v, err := t.isUp(); err != nil {
		return err
	} else if v {
		t.Log().Infof("container %s is already up", t.Name)
		return nil
	}
	if err := t.containerStart(ctx); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.Stop(ctx)
	})
	if !t.waitForUp(ctx, *t.StartTimeout, 2*time.Second) {
		return fmt.Errorf("waited too long for up")
	}
	if !t.hasEncap() {
		// No need to wait for ping exec access if we don't need to
		// execute anything in the vm.
		return nil
	}
	if !t.waitForPing(ctx, *t.StartTimeout, 2*time.Second) {
		return fmt.Errorf("waited too long for ping")
	}
	if !t.waitForOperational(ctx, *t.StartTimeout, 2*time.Second) {
		return fmt.Errorf("waited too long for operational")
	}
	return nil
}

func (t *T) Stop(ctx context.Context) error {
	if v, err := t.isDown(); err != nil {
		return err
	} else if v {
		t.Log().Infof("container %s is already down", t.Name)
		return nil
	}
	if err := t.containerStop(ctx); err != nil {
		return err
	}
	return nil
}

func (t *T) waitForDown(ctx context.Context, timeout, interval time.Duration) bool {
	t.Log().Attr("timeout", timeout).Infof("wait for %s shutdown (timeout %s)", t.Name, timeout)
	return waitfor.TrueCtx(ctx, timeout, interval, func() bool {
		v, err := t.isDown()
		if err != nil {
			return true
		}
		return v
	})
}

func (t *T) waitForUp(ctx context.Context, timeout, interval time.Duration) bool {
	t.Log().Attr("timeout", timeout).Infof("wait for %s up (timeout %s)", t.Name, timeout)
	return waitfor.TrueCtx(ctx, timeout, interval, func() bool {
		v, err := t.isUp()
		if err != nil {
			t.Log().Errorf("abort waiting for %s up: %s", t.Name, err)
			return true
		}
		return v
	})
}

func (t *T) waitForPing(ctx context.Context, timeout, interval time.Duration) bool {
	if t.QGA {
		return true
	}
	t.Log().Attr("timeout", timeout).Infof("wait for %s ping (timeout %s)", t.Name, timeout)
	return waitfor.TrueCtx(ctx, timeout, interval, func() bool {
		v, err := t.isPinging()
		if err != nil {
			t.Log().Errorf("abort waiting for %s ping: %s", t.Name, err)
			return true
		}
		return v
	})
}

func (t *T) waitForOperational(ctx context.Context, timeout, interval time.Duration) bool {
	t.Log().Attr("timeout", timeout).Infof("wait for %s operational (timeout %s)", t.Name, timeout)
	return waitfor.TrueCtx(ctx, timeout, interval, func() bool {
		v, err := t.isOperational()
		if err != nil {
			t.Log().Errorf("abort waiting for %s operational: %s", t.Name, err)
			return true
		}
		return v
	})
}

func (t *T) containerStop(ctx context.Context) error {
	state, err := t.domState()
	if err != nil {
		return err
	}
	switch state {
	case DomStateRunning:
		if err := t.stop(); err != nil {
			return err
		}
		if !t.waitForDown(ctx, *t.StopTimeout, 2*time.Second) {
			t.Log().Warnf("waited too long for shutdown")
			if err := t.destroy(); err != nil {
				return err
			}
		}
	case DomStateBlocked, DomStatePaused, DomStateCrashed:
		if err := t.destroy(); err != nil {
			return err
		}
	default:
		t.Log().Infof("skip stop, container state=%s", state)
		return nil
	}
	return nil
}

func (t *T) isUp() (bool, error) {
	state, err := t.domState()
	if err != nil {
		return false, err
	}
	return isUpFromState(state), nil
}

func isUpFromState(state string) bool {
	if state == DomStateRunning {
		return true
	}
	return false
}

func (t *T) isDown() (bool, error) {
	state, err := t.domState()
	if err != nil {
		return false, err
	}
	return isDownFromState(state), nil
}

func isDownFromState(state string) bool {
	switch state {
	case DomStateShutOff, DomStateNone:
		return true
	}
	return false
}

func (t *T) domState() (string, error) {
	cmd := command.New(
		command.WithName("virsh"),
		command.WithVarArgs("dominfo", t.Name),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithIgnoredExitCodes(0, 1),
	)
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	if strings.Contains(string(cmd.Stderr()), "failed to get domain") {
		return DomStateNone, nil
	}
	return domStateFromReader(bytes.NewReader(cmd.Stdout()))
}

func domStateFromReader(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		s := scanner.Text()
		if strings.HasPrefix(s, "State:") {
			return strings.TrimSpace(s[len("State:"):]), nil
		}
	}
	return "", fmt.Errorf("state not found")
}

func (t *T) hasConfigFile() bool {
	p := t.configFile()
	return file.Exists(p)
}

func (t *T) hasAutostartFile() bool {
	p := t.autostartFile()
	return file.Exists(p)
}

func (t *T) firmwareFiles() ([]string, error) {
	files := make([]string, 0)
	cf := t.configFile()
	f, err := os.Open(cf)
	if err != nil {
		return files, err
	}
	defer f.Close()
	doc, err := xmlquery.Parse(f)
	if err != nil {
		return files, err
	}

	es, err := xmlquery.QueryAll(doc, "//domain/os/nvram")
	if err != nil {
		return files, err
	}
	if len(es) > 0 {
		return files, nil
	}
	files = append(files, es[0].Data)

	es, err = xmlquery.QueryAll(doc, "//domain/os/loader")
	if err != nil {
		return files, err
	}
	if len(es) > 0 {
		return files, nil
	}
	files = append(files, es[0].Data)

	return files, nil
}

func (t *T) HasEFI() (bool, error) {
	cf := t.configFile()
	f, err := os.Open(cf)
	if err != nil {
		return false, err
	}
	defer f.Close()
	doc, err := xmlquery.Parse(f)
	if err != nil {
		return false, err
	}
	es, err := xmlquery.QueryAll(doc, "//domain/os/nvram")
	if err != nil {
		return false, err
	}
	if len(es) > 0 {
		return true, nil
	}
	es, err = xmlquery.QueryAll(doc, "//domain/os")
	if err != nil {
		return false, err
	}
	for _, e := range es {
		return e.SelectAttr("firmware") == "efi", nil
	}
	return false, nil
}

func (t *T) SubDevices() device.L {
	l := make(device.L, 0)
	cf := t.configFile()
	f, err := os.Open(cf)
	if err != nil {
		return l
	}
	defer f.Close()
	doc, err := xmlquery.Parse(f)
	if err != nil {
		return l
	}
	es, err := xmlquery.QueryAll(doc, "//domain/devices/disk")
	if err != nil {
		t.Log().Warnf("SubDevices: %s", err)
		return l
	}
	for _, e := range es {
		if dev := e.SelectAttr("dev"); dev != "" {
			l = append(l, device.New(dev))
		}
	}
	return l
}

func (t *T) setPartitions() error {
	cf := t.configFile()
	cgroupDir := t.cgroupDir()
	f, err := os.Open(cf)
	if err != nil {
		return err
	}
	defer f.Close()
	doc, err := xmlquery.Parse(f)
	if err != nil {
		return err
	}
	root, err := xmlquery.Query(doc, "//domain")
	if err != nil {
		return err
	}
	if root == nil {
		return fmt.Errorf("no <domain> node in %s", cf)
	}
	if n := root.SelectElement("//resource/partition"); n != nil {
		p := n.InnerText()
		if p != cgroupDir {
			t.Log().Infof("set text of //domain/resource/partition: %s", cgroupDir)
			partitionText := &xmlquery.Node{
				Data: cgroupDir,
				Type: xmlquery.TextNode,
			}
			n.FirstChild = partitionText
		}
	} else if resourceElem := root.SelectElement("//resource"); resourceElem != nil {
		t.Log().Infof("add to //domain/resource: <partition>%s</partition>", cgroupDir)
		partitionElem := &xmlquery.Node{
			Data: "partition",
			Type: xmlquery.ElementNode,
		}
		partitionText := &xmlquery.Node{
			Data: cgroupDir,
			Type: xmlquery.TextNode,
		}
		partitionElem.FirstChild = partitionText
		xmlquery.AddChild(resourceElem, partitionElem)
	} else {
		t.Log().Infof("add to //domain: <resource><partition>%s</partition></resource>", cgroupDir)
		resourceElem := &xmlquery.Node{
			Data: "resource",
			Type: xmlquery.ElementNode,
		}
		partitionElem := &xmlquery.Node{
			Data: "partition",
			Type: xmlquery.ElementNode,
		}
		partitionText := &xmlquery.Node{
			Data: cgroupDir,
			Type: xmlquery.TextNode,
		}
		partitionElem.FirstChild = partitionText
		resourceElem.FirstChild = partitionElem
		xmlquery.AddChild(root, resourceElem)

	}
	fmt.Println(doc.OutputXML(true))

	return nil
}

func (t *T) unsetPartitions() error {
	cf := t.configFile()
	f, err := os.Open(cf)
	if err != nil {
		return err
	}
	defer f.Close()
	doc, err := xmlquery.Parse(f)
	if err != nil {
		return err
	}
	e, err := xmlquery.Query(doc, "//domain/resource/partition")
	if err != nil {
		return err
	}
	if e != nil {
		t.Log().Infof("remove //domain/resource/partition")
		xmlquery.RemoveFromTree(e)
	}
	return nil
}

func (t *T) Status(ctx context.Context) status.T {
	/* TODO
	if pg := t.GetPG(); pg != nil && pg.IsFrozen() {
		t.StatusLog().Info("pg %s is frozen", pg)
		return status.NotApplicable
	}
	*/
	if t.hasAutostartFile() {
		t.StatusLog().Warn("container auto boot is on")
	}
	if !capabilities.Has(drvID.Cap()) {
		t.StatusLog().Info("this node is not kvm capable")
		return status.Undef
	}
	state, err := t.domState()
	if err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	}
	switch {
	case isUpFromState(state):
		return status.Up
	case isDownFromState(state):
		return status.Down
	default:
		t.StatusLog().Warn("dom state is %s", state)
		return status.Warn
	}
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	return t.Name
}

func (t *T) provisioned() (bool, error) {
	if state, err := t.domState(); err != nil {
		return false, err
	} else if state == DomStateNone {
		return false, nil
	} else {
		return true, nil
	}
}

func (t *T) UnprovisionAsFollower(ctx context.Context) error {
	isProvisioned, err := t.provisioned()
	if err != nil {
		return err
	}
	if !isProvisioned {
		t.Log().Infof("skip kvm unprovision: container is not provisioned")
		return nil
	}
	if t.hasConfigFile() {
		if err := t.undefine(); err != nil {
			return err
		}
	}
	return nil
}

func (t *T) UnprovisionAsLeader(ctx context.Context) error {
	if err := t.UnprovisionAsFollower(ctx); err != nil {
		return err
	}
	return nil
}

func (t *T) ProvisionAsLeader(ctx context.Context) error {
	isProvisioned, err := t.provisioned()
	if err != nil {
		return err
	}
	if isProvisioned {
		t.Log().Infof("skip kvm provision: container is provisioned")
		return nil
	}
	if len(t.VirtInst) == 0 {
		return fmt.Errorf("the 'virtinst' parameter must be set")
	}
	cmd := command.New(
		command.WithName(t.VirtInst[0]),
		command.WithArgs(t.VirtInst[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		//command.WithTimeout(*t.ProvisionTimeout),
	)
	return cmd.Run()
}

func (t *T) Unprovision(ctx context.Context) error {
	return nil
}

func (t *T) Provisioned() (provisioned.T, error) {
	if t.hasConfigFile() {
		return provisioned.True, nil
	}
	return provisioned.False, nil
}

func (t *T) rcmd() ([]string, error) {
	var args []string
	if len(t.RCmd) > 0 {
		args = t.RCmd
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("unable to identify a remote command method, install ssh or set the rcmd keyword")
	}
	args = append(args, t.GetHostname())
	return args, nil
}

func (t *T) rexec(cmd string) error {
	if rcmd, err := t.rcmd(); err == nil {
		rcmd = append(rcmd, cmd)
		return t.execViaRCmd(rcmd)
	}
	return t.execViaInternalSSH(cmd)
}

func (t *T) Enter() error {
	if rcmd, err := t.rcmd(); err == nil {
		return t.enterViaRCmd(rcmd)
	}
	return t.enterViaInternalSSH()
}

func (t *T) execViaInternalSSH(cmd string) error {
	hn := t.GetHostname()
	client, err := t.NewSSHClient(hn)
	if err != nil {
		return err
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	session.Stdin = os.Stdin
	if err := session.Run(cmd); err != nil {
		ee := err.(*ssh.ExitError)
		ec := ee.Waitmsg.ExitStatus()
		t.Log().
			Attr("exitcode", ec).
			Attr("cmd", cmd).
			Attr("host", hn).
			Debugf("rexec '%s' on host %s exited with code %d", cmd, hn, ec)
		return err
	}
	return nil
}

func (t *T) execViaRCmd(args []string) error {
	cmd := command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
		command.WithCommandLogLevel(zerolog.DebugLevel),
	)
	return cmd.Run()
}

func (t *T) enterViaInternalSSH() error {
	client, err := t.NewSSHClient(t.GetHostname())
	if err != nil {
		return err
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	termState, err := terminal.MakeRaw(int(os.Stdin.Fd()))
	defer terminal.Restore(int(os.Stdin.Fd()), termState)

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	session.Stdin = os.Stdin
	modes := ssh.TerminalModes{
		//	ssh.ECHO: 0,
	}
	width, height, err := terminal.GetSize(0)
	if err != nil {
		return err
	}
	if err := session.RequestPty("xterm", width, height, modes); err != nil {
		return err
	}
	if err := session.Shell(); err != nil {
		return err
	}
	_ = session.Wait()
	return nil
}

func (t *T) enterViaRCmd(rcmd []string) error {
	return syscall.Exec(rcmd[0], rcmd, os.Environ())
}

func (t *T) GetHostname() string {
	if t.Hostname != "" {
		return t.Hostname
	}
	return t.Name
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

/*
func (t *Path) ContainerHead() (string, error) {
	return t.rootDir()
}
*/

// cgroupDir returns the container resource cgroup path, relative to a controller head.
func (t *T) cgroupDir() string {
	return t.GetPGID()
}

func (t *T) Abort(ctx context.Context) bool {
	if v, err := t.isUp(); err != nil {
		t.Log().Warnf("no-abort: %s", err)
		return false
	} else if v {
		// the local instance is already up.
		// let the local start report the unnecessary start steps
		// but skip further abort tests
		return false
	} else {
		return t.abortPing() || t.abortPeerUp()
	}
}

func (t *T) abortPing() bool {
	hn := t.GetHostname()
	t.Log().Infof("abort test: ping %s", hn)

	if pinger, err := ping.NewPinger(hn); err == nil {
		pinger.Timeout = time.Second * 5
		pinger.Count = 1
		if err := pinger.Run(); err != nil {
			t.Log().Warnf("no-abort: pinger err: %s", err)
			return false
		}
		if pinger.Statistics().PacketsRecv > 0 {
			t.Log().Infof("abort: %s is alive", hn)
			return true
		}
		return false
	} else {
		t.Log().Debugf("disable ping abort check: %s", err)
	}
	return false
}

func (t *T) abortPeerUp() bool {
	if n, err := t.upPeer(); err != nil {
		return false
	} else if n != "" {
		t.Log().Infof("abort: %s is up on %s", t.GetHostname(), n)
		return true
	}
	return false
}

func (t *T) upPeer() (string, error) {
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
		cmd := fmt.Sprintf("virsh dominfo %s", t.Name)
		err = session.Run(cmd)
		if err != nil {
			ee := err.(*ssh.ExitError)
			ec := ee.Waitmsg.ExitStatus()
			return ec == 0, err
		}
		state, err := domStateFromReader(io.Reader(&b))
		if err != nil {
			return false, err
		}
		return isUpFromState(state), err
	}
	for _, n := range t.Peers {
		if n == t.Hostname {
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
	if t.QGA {
		return t.EncapCmdWithQGA(ctx, args, envs)
	} else {
		return t.EncapCmdWithRCmd(ctx, args, envs)
	}
}

func (t *T) EncapCmdWithQGA(ctx context.Context, args []string, envs []string) (*qgaCommand, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("EncapCmdWithQGA call with empty a 'args []string' argument")
	}
	cmd := newQGACommand(ctx, t.Name, args[0], args[1:], envs)
	return cmd, nil
}

func (t *T) EncapCmdWithRCmd(ctx context.Context, args []string, envs []string) (*exec.Cmd, error) {
	baseArgs, err := t.rcmd()
	if err != nil {
		return nil, err
	}
	baseArgs = append(baseArgs, envs...)
	baseArgs = append(baseArgs, args...)
	cmd := exec.CommandContext(ctx, baseArgs[0], baseArgs[1:]...)
	return cmd, nil
}

func (t *T) rcmdCp(ctx context.Context, src, dst string) error {
	baseArgs, err := t.rcmd()
	if err != nil {
		return err
	}
	baseArgs[0] = strings.Replace(baseArgs[0], "ssh", "scp", 1)
	baseArgs = append(baseArgs[:len(baseArgs)-1], src, t.GetHostname()+":"+dst)
	cmd := exec.CommandContext(ctx, baseArgs[0], baseArgs[1:]...)
	return cmd.Run()
}

func (t *T) EncapCp(ctx context.Context, src, dst string) error {
	if t.QGA {
		return qgaCp(ctx, t.Name, src, dst)
	}
	return t.rcmdCp(ctx, src, dst)
}

func (t *T) GetOsvcRootPath() string {
	if t.OsvcRootPath != "" {
		return filepath.Join(t.OsvcRootPath, "bin", "om")
	}
	return filepath.Join(rawconfig.Paths.Bin, "om")
}
