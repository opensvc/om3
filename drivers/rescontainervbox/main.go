package rescontainervbox

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/antchfx/xmlquery"
	"github.com/go-ping/ping"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/opensvc/om3/core/actionrollback"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/core/topology"
	"github.com/opensvc/om3/util/capabilities"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/device"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/sshnode"
)

type (
	T struct {
		resource.T
		resource.SCSIPersistentReservation
		Path     path.T    `json:"path"`
		ObjectID uuid.UUID `json:"object_id"`
		Peers    []string  `json:"peers"`
		DNS      []string  `json:"dns"`
		Topology topology.T

		Headless       bool `json:"headless"`
		SCSIReserv     bool `json:"scsireserv"`
		PromoteRW      bool `json:"promote_rw"`
		NoPreemptAbort bool `json:"no_preempt_abort"`

		OsvcRootPath string         `json:"osvc_root_path"`
		GuestOS      string         `json:"guest_os"`
		Name         string         `json:"name"`
		Hostname     string         `json:"hostname"`
		RCmd         []string       `json:"rcmd"`
		StartTimeout *time.Duration `json:"start_timeout"`
		StopTimeout  *time.Duration `json:"stop_timeout"`

		cache map[string]interface{}
	}

	header interface {
		Head() string
	}
	resourceLister interface {
		Resources() resource.Drivers
	}
)

var (
	ErrNotRegistered = errors.New("vm is not registered")
)

func New() resource.Driver {
	t := &T{
		cache: make(map[string]interface{}),
	}
	return t
}

func (t *T) Abort(ctx context.Context) bool {
	if isLocalUp, err := t.isUp(); errors.Is(err, ErrNotRegistered) {
		t.Log().Debug().Err(err).Send()
	} else if err != nil {
		t.Log().Error().Err(err).Send()
	} else if isLocalUp {
		// the local instance is already up.
		// let the local start report the unecessary start steps
		// but skip further abort tests
		return false
	}
	return t.abortPing() || t.abortPeerUp()
}

func (t *T) Enter() error {
	if rcmd, err := t.rcmd(); err == nil {
		return t.enterViaRCmd(rcmd)
	}
	return t.enterViaInternalSSH()
}

func (t *T) Label() string {
	return t.Name
}

func (t *T) Start(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, *t.StartTimeout)
	defer cancel()
	if err := t.ApplyPGChain(ctx); err != nil {
		return err
	}

	if isContainerUp, err := t.isUp(); errors.Is(err, ErrNotRegistered) {
		if err := t.registerVm(); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if isContainerUp {
		t.Log().Info().Msgf("container %s is already up", t.Name)
		return nil
	}

	if err := t.containerStart(ctx); err != nil {
		return err
	}
	actionrollback.Register(ctx, func() error {
		return t.Stop(ctx)
	})
	if err := t.waitForExpectation(ctx, "up", true, t.isUp); err != nil {
		return err
	}

	if _, err := net.LookupIP(t.hostname()); err != nil {
		t.Log().Debug().Msgf("can not do dns resolution for : %s", t.Name)
		return nil
	}
	if err := t.waitForExpectation(ctx, "ping", true, t.isPinging); err != nil {
		return err
	}
	if err := t.waitForExpectation(ctx, "operational", true, t.isOperational); err != nil {
		return err
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
	if !capabilities.Has(drvID.Cap()) {
		t.StatusLog().Info("this node is not vbox capable")
		return status.Undef
	}

	state, err := t.domState()
	if err != nil {
		if errors.Is(err, ErrNotRegistered) {
			return status.Down
		}
		t.StatusLog().Error("%s", err)
		return status.Undef
	}
	switch {
	case isUpFromState(state):
		return status.Up
	case isDownFromState(state):
		return status.Down
	case isAbortedFromState(state):
		t.StatusLog().Warn("dom state is aborted")
		return status.Down
	default:
		t.StatusLog().Warn("dom state is %s", state)
		return status.Warn
	}
}

func (t *T) Stop(ctx context.Context) error {
	if isContainerDown, err := t.isDown(); errors.Is(err, ErrNotRegistered) {
		t.Log().Info().Msgf("container %s is already down (not registered)", t.Name)
		return nil
	} else if err != nil {
		return err
	} else if isContainerDown {
		t.Log().Info().Msgf("container %s is already down", t.Name)
		return nil
	}
	return t.containerStop(ctx)
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
	for _, e := range xmlquery.Find(doc, "//domain/devices/disk") {
		if dev := e.SelectAttr("dev"); dev != "" {
			l = append(l, device.New(dev))
		}
	}
	return l
}

func (t *T) Presync() error {
	vboxCfgFilePath := t.getVBoxCfgFile()
	f, err := os.Create(vboxCfgFilePath)
	if err != nil {
		return err
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Log().Error().Err(err).Msgf(vboxCfgFilePath+" %s", "defer close")
		}
	}()
	_, err = f.WriteString(t.configFile())
	return err
}

func (t *T) ToSync() []string {
	if t.Topology == topology.Failover && !t.IsShared() {
		return t.configFiles()
	}
	return []string{}
}

func (t *T) vBoxManageCommand(args ...string) (string, error) {
	cmd := command.New(
		command.WithName("VBoxManage"),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
	)
	err := cmd.Run()

	if strings.Contains(string(cmd.Stderr()), "0x80bb0001") {
		return string(cmd.Stdout()), fmt.Errorf("%w:%w", err, ErrNotRegistered)
	}
	return string(cmd.Stdout()), err
}

func (t *T) getVBoxCfgFile() string {
	return filepath.Join(t.VarDir(), "vboxcfgfile")
}
func (t *T) readConfigFileFromVarDir() (string, error) {
	f, err := os.ReadFile(t.getVBoxCfgFile())
	if err != nil {
		return "", fmt.Errorf("can't find config file: %w", err)
	}
	return string(f), nil
}

func (t *T) configFile() string {
	t.Log().Info().Msgf("VBoxManage showvminfo --machinereadable %s", t.Name)
	b, err := t.vBoxManageCommand("showvminfo", "--machinereadable", t.Name)
	if err != nil {
		t.Log().Error().Msgf("can't find config file: %s", err)
		return ""
	}
	if cfgFile, err := configFileFromReader(strings.NewReader(b)); err != nil {
		t.Log().Error().Msgf("can't find cfgfile in showvminfo command: %s", err)
		return ""
	} else {
		return cfgFile
	}
}

func configFileFromReader(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		s := scanner.Text()
		if strings.HasPrefix(s, "CfgFile=") {
			return strings.Trim(s[len("CfgFile="):], "\""), nil
		}
	}

	return "", fmt.Errorf("config file not found")
}

func (t *T) configFiles() []string {
	cf := t.configFile()
	if !file.Exists(cf) {
		return []string{}
	}
	return []string{cf}
}

func (t *T) checkCapabilities() bool {
	if !capabilities.Has(drvID.Cap() + ".hvm") {
		t.StatusLog().Warn("hvm not supported by host")
		return false
	}
	return true
}

func (t *T) isOperational() (bool, error) {
	if err := t.rexec("pwd"); err != nil {
		t.Log().Debug().Err(err).Msgf("isOperational")
		return false, nil
	}
	return true, nil
}

func (t *T) isPinging() (bool, error) {
	pinger, err := ping.NewPinger(t.hostname())
	if err != nil {
		return false, err
	}
	pinger.Timeout = time.Second * 1
	pinger.Count = 1
	pinger.SetPrivileged(true)
	if err := pinger.Run(); err != nil {
		return false, err
	}
	if pinger.Statistics().PacketsRecv > 0 {
		return true, nil
	}
	return false, nil
}

func (t *T) undefine() error {
	cmd := command.New(
		command.WithName("virsh"),
		command.WithVarArgs("undefine", t.Name),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}

func (t *T) start() error {
	t.Log().Info().Msgf("VBoxManage startvm %s --type=headless", t.Name)
	_, err := t.vBoxManageCommand("startvm", t.Name, "--type=headless")
	return err
}

func (t *T) stop() error {
	t.Log().Info().Msgf("VBoxManage controlvm %s acpipowerbutton", t.Name)
	_, err := t.vBoxManageCommand("controlvm", t.Name, "acpipowerbutton")
	return err
}

func (t *T) destroy() error {
	t.Log().Info().Msgf("VBoxManage controlvm %s poweroff", t.Name)
	_, err := t.vBoxManageCommand("controlvm", t.Name, "poweroff")
	return err
}

func (t *T) containerStart(ctx context.Context) error {
	if !t.hasConfigFile() {
		return fmt.Errorf("%s not found", t.configFile())
	}
	return t.start()
}

func (t *T) containerStop(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, *t.StopTimeout)
	defer cancel()
	state, err := t.domState()
	if err != nil {
		return err
	}
	switch state {
	case "running":
		if err := t.stop(); err != nil {
			return err
		}
		if err := t.waitForExpectation(ctx, "shutdown", false, t.isDown); err != nil {
			t.Log().Warn().Msg("waited too long for shutdown")
			return t.destroy()
		}
		return nil
	case "stuck", "paused", "aborted":
		return t.destroy()
	case "poweroff":
		t.Log().Info().Msgf("skip stop, container state=%s", state)
		return nil
	default:
		err := fmt.Errorf("container stop found unexpected state %s", state)
		t.Log().Error().Err(err).Msgf("don't know how to stop vm")
		return err
	}
}

func (t *T) registerVm() error {
	configFilePath, err := t.readConfigFileFromVarDir()
	if err != nil {
		return err
	}
	if configFilePath == "" {
		return fmt.Errorf("can't register: vm unknown config file path")
	}
	t.Log().Info().Msgf("VBoxManage registervm %s", configFilePath)
	_, err = t.vBoxManageCommand("registervm", configFilePath)
	return err
}

func (t *T) waitForExpectation(ctx context.Context, s string, logError bool, fn func() (bool, error)) error {
	t.Log().Info().Msgf("wait for %s %s", s, t.Name)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if ok, err := fn(); err != nil {
				if logError {
					t.Log().Error().Err(err).Msgf("abort waiting for %s %s", t.Name, s)
				}
				return nil
			} else if ok {
				return nil
			}
		}
	}
}

func (t *T) isUp() (bool, error) {
	state, err := t.domState()
	if err != nil {
		return false, err
	}
	return isUpFromState(state), nil
}

func isUpFromState(state string) bool {
	if state == "running" {
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
	case "poweroff":
		return true
	}
	return false
}

func isAbortedFromState(state string) bool {
	switch state {
	case "aborted":
		return true
	}
	return false
}

func (t *T) domState() (string, error) {
	t.Log().Debug().Msgf("VBoxManage showvminfo --machinereadable %s", t.Name)
	s, err := t.vBoxManageCommand("showvminfo", "--machinereadable", t.Name)
	if err != nil {
		return "", err
	}
	return domStateFromReader(strings.NewReader(s))
}

func domStateFromReader(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		s := scanner.Text()
		if strings.HasPrefix(s, "VMState=") {
			return strings.Trim(s[len("VMState="):], "\""), nil
		}
	}
	return "", fmt.Errorf("state not found")
}

func (t *T) hasConfigFile() bool {
	p := t.configFile()
	return file.Exists(p)
}

func (t *T) rcmd() ([]string, error) {
	if len(t.RCmd) > 0 {
		return t.RCmd, nil
	}
	return nil, fmt.Errorf("unable to identify a remote command method. install ssh or set the rcmd keyword.")
}

func (t *T) rexec(cmd string) error {
	if rcmd, err := t.rcmd(); err == nil {
		rcmd = append(rcmd, cmd)
		return t.execViaRCmd(rcmd)
	}
	return t.execViaInternalSSH(cmd)
}

func (t *T) execViaInternalSSH(cmd string) error {
	hn := t.hostname()
	client, err := sshnode.NewClient(hn)
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
	if err != nil {
		return err
	}
	if err := session.Run(cmd); err != nil {
		ee := err.(*ssh.ExitError)
		ec := ee.Waitmsg.ExitStatus()
		t.Log().Debug().Int("exitcode", ec).Str("cmd", cmd).Str("host", hn).Msg("rexec")
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
	client, err := sshnode.NewClient(t.hostname())
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
	sh := "/bin/bash"
	args := append(rcmd, sh)
	cmd := exec.Command(args[0], args[1:]...)
	_ = cmd.Run()

	switch cmd.ProcessState.ExitCode() {
	case 126, 127:
		sh = "/bin/sh"
	}
	args = append(rcmd, sh)
	return syscall.Exec(args[0], args, os.Environ())
}

func (t *T) hostname() string {
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

// cgroupDir returns the container resource cgroup path, relative to a controler head.
func (t *T) cgroupDir() string {
	return t.GetPGID()
}

func (t *T) abortPing() bool {
	hn := t.hostname()
	t.Log().Info().Msgf("abort test: ping %s", hn)

	if pinger, err := ping.NewPinger(hn); err == nil {
		pinger.SetPrivileged(true)
		pinger.Timeout = time.Second * 5
		pinger.Count = 1
		if err := pinger.Run(); err != nil {
			t.Log().Warn().Msgf("no-abort: pinger err: %s", err)
			return false
		}
		if pinger.Statistics().PacketsRecv > 0 {
			t.Log().Info().Msgf("abort: %s is alive", hn)
			return true
		}
		return false
	} else {
		t.Log().Debug().Msgf("disable ping abort check: %s", err)
	}
	return false
}

func (t *T) abortPeerUp() bool {
	if n, err := t.upPeer(); err != nil {
		return false
	} else if n != "" {
		t.Log().Info().Msgf("abort: %s is up on %s", t.hostname(), n)
		return true
	}
	return false
}

func (t *T) upPeer() (string, error) {
	isPeerUp := func(n string) (bool, error) {
		client, err := sshnode.NewClient(n)
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
		cmd := fmt.Sprintf("VBoxManage showvminfo --machinereadable %s", t.Name)
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
			t.Log().Debug().Msgf("ssh abort check on %s: %s", n, err)
			continue
		} else if v {
			return n, nil
		}
	}
	return "", nil
}
