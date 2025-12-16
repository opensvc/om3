package daemoncmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime/pprof"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/gommon/log"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/hbsecobject"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/daemon"
	"github.com/opensvc/om3/v3/daemon/daemonsys"
	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/lock"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/waitfor"
)

var (
	lockPath           = "/tmp/locks/main"
	lockTimeout        = 60 * time.Second
	WaitRunningTimeout = 20 * time.Second
	WaitRunningDelay   = 500 * time.Millisecond
	WaitStoppedTimeout = 4 * time.Second
	WaitStoppedDelay   = 250 * time.Millisecond
	errGoTest          = errors.New("running from go test")

	ErrAlreadyRunning = errors.New("daemon already running")
)

type (
	T struct {
		client    *client.T
		node      string
		daemonsys Manager
	}

	Manager interface {
		Activated(ctx context.Context) (bool, error)
		CalledFromManager() bool
		Close() error
		Defined(ctx context.Context) (bool, error)
		Start(ctx context.Context) error
		Restart() error
		Stop(context.Context) error
		IsSystemStopping() (bool, error)
	}

	starter interface {
		Start(context.Context) error
	}
)

func bootStrapCcfg() error {
	log := logger("bootstrap cluster config: ")
	type mandatoryKeyT struct {
		Key       key.T
		Default   string
		Obfuscate bool
	}
	keys := []mandatoryKeyT{
		{
			Key:       key.New("cluster", "id"),
			Default:   uuid.New().String(),
			Obfuscate: false,
		},
		{
			Key:       key.New("cluster", "name"),
			Default:   naming.Random(),
			Obfuscate: false,
		},
		{
			Key:       key.New("cluster", "nodes"),
			Default:   hostname.Hostname(),
			Obfuscate: false,
		},
		{
			Key:       key.New("cluster", "secret"),
			Default:   strings.ReplaceAll(uuid.New().String(), "-", ""),
			Obfuscate: true,
		},
	}

	ccfg, err := object.NewCluster(object.WithVolatile(false))
	if err != nil {
		return err
	}

	for _, k := range keys {
		if ccfg.Config().Get(k.Key) != "" {
			continue
		}
		op := keyop.New(k.Key, keyop.Set, k.Default, 0)
		if err := ccfg.Config().PrepareSet(*op); err != nil {
			return err
		}
		if k.Obfuscate {
			op.Value = "xxxx"
		}
		log.Infof("%s", op)
	}

	// Prepares futures node join, because it requires at least one heartbeat.
	// So on cluster config where no hb exists, we automatically set hb#1.type=unicast.
	hasHbSection := false
	for _, section := range ccfg.Config().SectionStrings() {
		if strings.HasPrefix(section, "hb") {
			hasHbSection = true
			break
		}
	}
	if !hasHbSection {
		k := key.New("hb#1", "type")
		op := keyop.New(k, keyop.Set, "unicast", 0)
		if err := ccfg.Config().PrepareSet(*op); err != nil {
			return err
		}
		log.Infof("add default heartbeat: %s", op)
	}

	if err := ccfg.Config().Commit(); err != nil {
		return err
	}
	if cfg, err := object.SetClusterConfig(); err != nil {
		return err
	} else {
		for _, issue := range cfg.Issues {
			log.Warnf("issue: %s", issue)
		}
	}

	if secret := ccfg.Config().Get(key.New("cluster", "secret")); secret != "" {
		if err := bootStrapSecHb(secret, 0); err != nil {
			return err
		}
	}
	return nil
}

func bootStrapSecHb(currentSecret string, currentVersion uint64) error {
	log := logger("bootstrap heartbeat secret")
	type keyT struct {
		Name      string
		Value     string
		Obfuscate bool
	}
	keys := []keyT{
		{
			Name:      hbsecobject.Secret,
			Value:     currentSecret,
			Obfuscate: true,
		},
		{
			Name:      hbsecobject.Version,
			Value:     fmt.Sprintf("%d", currentVersion),
			Obfuscate: false,
		},
		{
			Name:      hbsecobject.AltSecret,
			Value:     currentSecret,
			Obfuscate: true,
		},
		{
			Name:      hbsecobject.AltVersion,
			Value:     fmt.Sprintf("%d", currentVersion),
			Obfuscate: false,
		},
	}

	secHb, err := object.NewSec(naming.SecHb, object.WithVolatile(false))
	if err != nil {
		return err
	}

	existingKeys, err := secHb.AllKeys()
	if err != nil {
		return err
	}
	var changed bool
	for _, k := range keys {
		if slices.Contains(existingKeys, k.Name) {
			// already exists, skipped
			continue
		}
		log.Infof("%s adding key %s", secHb, k.Name)
		if err := secHb.TransactionAddKey(k.Name, []byte(k.Value)); err != nil {
			return fmt.Errorf("can't add key %s: %w", k.Name, err)
		}
		changed = true
	}
	if changed {
		log.Infof("%s commit changes ...", secHb)
		if err := secHb.Config().Commit(); err != nil {
			return fmt.Errorf("can't commit %s changes: %w", secHb, err)
		}
	}
	return nil
}

func New(c *client.T) *T {
	return &T{client: c}
}

func (t *T) LoadManager(ctx context.Context) error {
	var i any
	i, err := daemonsys.New(ctx)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// skip: Error: dial unix /run/systemd/private: connect: no such file or directory
			return nil
		}
		return err
	}
	if mgr, ok := i.(Manager); ok {
		t.daemonsys = mgr
	}
	return nil
}

// Restart handle daemon restart from command origin.
//
// It is used to forward restart control to (systemd) manager (when the origin is not systemd)
func (t *T) Restart(ctx context.Context, profile string) error {
	if t.daemonsys == nil {
		return t.restartWithoutManager(profile)
	}
	defer func() {
		_ = t.daemonsys.Close()
	}()
	if ok, err := t.daemonsys.Defined(ctx); err != nil || !ok {
		return t.restartWithoutManager(profile)
	}
	// note: always ask manager for restart (during POST /daemon/restart handler
	// the server api is probably CalledFromManager). And systemd unit doesn't define
	// restart command.
	return t.restartWithManager()
}

func (t *T) SetNode(node string) {
	t.node = node
}

// run function will start daemon with internal lock protection
func (t *T) run(ctx context.Context) error {
	log := logger("locked run: ")
	if err := rawconfig.CreateMandatoryDirectories(); err != nil {
		return fmt.Errorf("create mandatory directories: %w", err)
	}
	release, err := getLock("Run")
	if err != nil {
		return err
	}
	isRunning, err := t.isRunning()
	if err != nil {
		return err
	}
	if isRunning {
		return ErrAlreadyRunning
	}
	pidFile := daemonPidFile()
	log.Tracef("create pid file %s", pidFile)

	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644); err != nil {
		return err
	}
	defer func() {
		log.Tracef("remove pid file %s", pidFile)
		if err := os.Remove(pidFile); err != nil {
			log.Errorf("remove pid file %s: %s", pidFile, err)
		}
	}()
	if err := capabilities.Scan(ctx); err != nil {
		return err
	}
	log.Attr("capabilities", capabilities.Data()).Infof("rescanned node capabilities")

	if err := bootStrapCcfg(); err != nil {
		log.Tracef("bootstrap cluster config %s", err)
		return err
	}
	d := daemon.New()
	log.Tracef("starting daemon...")
	err = d.Start(context.Background())
	release()
	if err != nil {
		return err
	}
	if d != nil {
		log.Infof("started")
		d.Wait()
		log.Infof("stopped")
	}
	return nil
}

// Start handle daemon start from command origin.
//
// It is used to forward start control to (systemd) manager (when the origin is not systemd)
func (t *T) Start(ctx context.Context, profile string) error {
	if t.daemonsys == nil {
		return t.startWithoutManager(profile)
	}
	defer func() {
		_ = t.daemonsys.Close()
	}()
	if ok, err := t.daemonsys.Defined(ctx); err != nil || !ok {
		return t.startWithoutManager(profile)
	} else if t.daemonsys.CalledFromManager() {
		return t.startFromManager(profile)
	} else {
		return t.startWithManager(ctx)
	}
}

func (t *T) startFromManager(profile string) error {
	if isRunning, err := t.isRunning(); err != nil {
		return err
	} else if isRunning {
		return nil
	}
	log.Infof("exec run (origin manager)")
	return t.start(profile)
}

// Stop handle daemon stop from command origin.
//
// It is used to forward stop control to (systemd) manager (when the origin is not systemd)
func (t *T) Stop(ctx context.Context) error {
	if t.daemonsys == nil {
		return t.StopWithoutManager()
	}
	defer func() {
		_ = t.daemonsys.Close()
	}()
	if ok, err := t.daemonsys.Defined(ctx); err != nil || !ok {
		return t.StopWithoutManager()
	}
	if t.daemonsys.CalledFromManager() {
		return t.stopFromManager()
	}
	return t.stopWithManager(ctx)
}

// IsRunning function detect daemon status using api
//
// it returns true is daemon is running, else false
func (t *T) IsRunning() (bool, error) {
	return t.isRunning()
}

// WaitRunning function waits for daemon running
//
// It needs to be called from a cli lock protection
func (t *T) WaitRunning() error {
	if ok, err := waitfor.TrueNoError(WaitRunningTimeout, WaitRunningDelay, t.IsRunning); err != nil {
		return fmt.Errorf("wait running: %s", err)
	} else if !ok {
		return fmt.Errorf("wait running: timeout")
	}
	return nil
}

// getLock() manage internal lock for functions that will stop/start/restart daemon
//
// It returns a release function to release lock
func getLock(desc string) (func(), error) {
	return lock.Lock(lockPath, lockTimeout, desc)
}

// lockCmdCheck starts cmd, then call checker() with cli lock protection
func lockCmdCheck(cmd *command.T, checker func() error, desc string) error {
	f := func() error {
		if err := cmd.Start(); err != nil {
			return err
		}
		if checker != nil {
			if err := checker(); err != nil {
				return err
			}
		}
		return nil
	}
	if err := lock.Func(lockPath+"-cli", 60*time.Second, desc, f); err != nil {
		return err
	}
	return nil
}

// lockFuncAndCheck starts cmd, then call checker() with cli lock protection
func lockFuncAndCheck(ctx context.Context, starter starter, checker func() error, desc string) error {
	f := func() error {
		if err := starter.Start(ctx); err != nil {
			return err
		}
		if checker != nil {
			if err := checker(); err != nil {
				return err
			}
		}
		return nil
	}
	if err := lock.Func(lockPath+"-cli", 60*time.Second, desc, f); err != nil {
		return err
	}
	return nil
}

// StopWithoutManager function will stop daemon with internal lock protection
func (t *T) StopWithoutManager() error {
	release, err := getLock("Stop")
	if err != nil {
		return err
	}
	defer release()
	return t.stop()
}

func (t *T) start(profile string) error {
	if isRunning, err := t.isRunning(); err != nil {
		return err
	} else if isRunning {
		return ErrAlreadyRunning
	}
	checker := func() error {
		if err := t.WaitRunning(); err != nil {
			return fmt.Errorf("start checker wait running failed: %w", err)
		}
		return nil
	}
	args := []string{"daemon", "run"}
	if profile != "" {
		args = append(args, "--cpuprofile", profile)
	}
	cmd := command.New(
		command.WithName(os.Args[0]),
		command.WithArgs(args),
	)
	return lockCmdCheck(cmd, checker, "daemon start")
}

func (t *T) startWithoutManager(profile string) error {
	if isRunning, err := t.isRunning(); err != nil {
		return err
	} else if isRunning {
		return nil
	}
	return t.start(profile)
}

func (t *T) startWithManager(ctx context.Context) error {
	if isRunning, err := t.isRunning(); err != nil {
		return err
	} else if isRunning {
		return ErrAlreadyRunning
	}
	if err := t.daemonsys.Start(ctx); err != nil {
		return fmt.Errorf("daemonsys start: %w", err)
	}
	if err := t.WaitRunning(); err != nil {
		return fmt.Errorf("wait running: %w", err)
	}
	return nil
}

func (t *T) restartWithoutManager(profile string) error {
	if err := t.StopWithoutManager(); err != nil {
		return err
	}
	return t.startWithoutManager(profile)
}

func (t *T) restartWithManager() error {
	if err := t.daemonsys.Restart(); err != nil {
		return fmt.Errorf("daemonsys restart: %w", err)
	}
	return nil
}

func (t *T) stopWithManager(ctx context.Context) error {
	if ok, err := t.daemonsys.Activated(ctx); err != nil {
		err := fmt.Errorf("can't detect activated state: %w", err)
		return err
	} else if !ok {
		// recover inconsistent manager view not activated, but reality is running
		if err := t.StopWithoutManager(); err != nil {
			return fmt.Errorf("failed during recover: %w", err)
		}
	} else {
		if err := t.daemonsys.Stop(ctx); err != nil {
			return fmt.Errorf("daemonsys stop: %w", err)
		}
	}
	return nil
}

func (t *T) stopFromManager() error {
	isSystemStopping, err := t.daemonsys.IsSystemStopping()
	if err != nil {
		return err
	}

	if isSystemStopping {
		fmt.Printf("the operating system is stopping: promote to daemon shutdown")
		cmd := command.New(
			command.WithName(os.Args[0]),
			command.WithVarArgs("daemon", "shutdown"),
		)
		cmd.Cmd().Stdout = io.Discard
		cmd.Cmd().Stderr = io.Discard
		return cmd.Run()
	}
	return t.StopWithoutManager()
}

func (t *T) stop() error {
	log := logger("stop: ")
	resp, err := t.client.PostDaemonStopWithResponse(context.Background(), hostname.Hostname())
	if err != nil {
		if !errors.Is(err, syscall.ECONNRESET) &&
			!strings.Contains(err.Error(), "unexpected EOF") &&
			!strings.Contains(err.Error(), "unexpected end of JSON input") {
			log.Errorf("post daemon stop: %s, kill", err)
			return t.kill()
		}
		return err
	}
	switch {
	case resp.JSON200 != nil:
		log.Tracef("wait for stop...")
		pid := resp.JSON200.Pid
		hasProcfile := func() (bool, error) {
			return t.hasProcFile(pid)
		}
		if ok, err := waitfor.FalseNoError(WaitStoppedTimeout, WaitStoppedDelay, hasProcfile); err != nil {
			log.Warnf("daemon pid %d wait not running: %s, kill", pid, err)
			return t.kill()
		} else if !ok {
			log.Warnf("daemon pid %d still running after stop request, kill", pid)
			return t.kill()
		}
		log.Tracef("stopped")
		// one more delay before return listener not anymore responding
		time.Sleep(WaitStoppedDelay)
	default:
		log.Warnf("unexpected status code: %s body: %s, kill", resp.Status(), string(resp.Body))
		time.Sleep(WaitStoppedDelay)
		return t.kill()
	}

	return nil
}

func (t *T) startProfile(profile string) (func(), error) {
	f, err := os.Create(profile)
	if err != nil {
		return nil, fmt.Errorf("create CPU profile: %w", err)
	}
	stop := func() {
		_ = f.Close()
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		return stop, fmt.Errorf("start CPU profile: %w", err)
	}
	stop = func() {
		pprof.StopCPUProfile()
		_ = f.Close()
	}
	return stop, nil
}

func (t *T) Run(ctx context.Context, profile string) error {
	if profile != "" {
		if stopProfile, err := t.startProfile(profile); err != nil {
			return err
		} else {
			defer stopProfile()
		}
	}
	if err := t.run(ctx); err != nil {
		return err
	}
	return nil
}

func (t *T) kill() error {
	pid, err := t.getPid()
	if errors.Is(err, errGoTest) {
		return nil
	}
	if pid <= 0 {
		return nil
	}
	return syscall.Kill(pid, syscall.SIGKILL)
}

func (t *T) hasProcFile(pid int) (bool, error) {
	filename := fmt.Sprintf("/proc/%d", pid)
	_, err := os.Stat(filename)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// isRunning returns true if the daemon api responds or if process id from
// daemon pid file exists, matching 'daemon run' and is not self pid.
func (t *T) isRunning() (bool, error) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	resp, err := t.client.GetNodePing(ctx, hostname.Hostname())
	if err == nil && resp.StatusCode == http.StatusNoContent {
		return true, nil
	}

	pid, err := t.getPid()
	if errors.Is(err, errGoTest) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if pid < 0 {
		return false, nil
	}

	if os.Getpid() == pid {
		// avoids false positives caused by remnants of a previous daemon PID
		// after a fast OS reboot.
		return false, nil
	}
	hasProcFile, err := t.hasProcFile(pid)
	if err != nil {
		return false, err
	}
	return hasProcFile, err
}

func (t *T) getPid() (int, error) {
	pidFile := daemonPidFile()
	pid, err := extractPidFromPidFile(pidFile)
	if errors.Is(err, os.ErrNotExist) {
		return -1, nil
	} else if errors.Is(err, syscall.ESRCH) {
		return -1, nil
	} else if err != nil {
		return -1, err
	}
	v, err := isCmdlineMatchingDaemon(pid)
	if !v {
		return -1, err
	}
	return pid, err
}

func isCmdlineMatchingDaemon(pid int) (bool, error) {
	log := logger("validate proc: ")
	getPidInfo := func(pid int) (args []string, running bool, err error) {
		b, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))

		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		} else if errors.Is(err, syscall.ESRCH) {
			return nil, false, nil
		} else if err != nil {
			return nil, false, err
		} else if strings.Contains(string(b), "/daemoncmd.test") {
			return nil, true, errGoTest
		}
		if len(b) == 0 {
			return nil, false, nil
		}
		sep := make([]byte, 1)
		l := bytes.Split(b, sep)
		args = make([]string, len(l))
		for i, b := range l {
			args[i] = string(b)
		}
		return args, true, nil
	}

	areProcArgsMatching := func(args []string) bool {
		if len(args) == 0 {
			log.Tracef("process %d pointed by %s ran by a command with no arguments", pid, daemonPidFile())
			return false
		}
		if len(args) < 3 {
			log.Tracef("process %d pointed by %s ran by a command with too few arguments: %s", pid, daemonPidFile(), args)
			return false
		}
		if args[1] != "daemon" || args[2] != "run" {
			log.Tracef("process %d pointed by %s is not a om daemon: %s", pid, daemonPidFile(), args)
			return false
		}
		return true
	}

	if l, running, err := getPidInfo(pid); err != nil {
		return false, err
	} else if !running {
		return false, nil
	} else if len(l) == 0 {
		// need rescan, pid is detected, but read the read /proc/%d/cmdline may returns empty []byte
		//     om[364661]: daemon: main: daemon started
		//     om[364661]: daemon: cmd: locked start: started
		//     ...
		//     om[368219]: daemon: cmd: cli restart: origin os, no unit defined
		//     om[364661]: daemon: main: stopping on daemon ctl message
		//     om[364661]: daemon: main: daemon stopping
		//     om[368219]: daemon: cmd: start from cmd: wait running: start checker wait
		//                 running failed: wait running: process 364661 pointed
		//                 by /var/lib/opensvc/osvcd.pid ran by a command with too few arguments: []
		time.Sleep(500 * time.Millisecond)
		if l, running, err := getPidInfo(pid); err != nil {
			return false, err
		} else if !running {
			return false, nil
		} else {
			return areProcArgsMatching(l), nil
		}
	} else {
		return areProcArgsMatching(l), nil
	}
}

func extractPidFromPidFile(pidFile string) (int, error) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return -1, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return -1, err
	}
	return pid, nil
}

func daemonPidFile() string {
	return filepath.Join(rawconfig.Paths.Var, "osvcd.pid")
}

func logger(s string) *plog.Logger {
	return plog.NewDefaultLogger().
		Attr("pkg", "daemon/daemoncmd").
		WithPrefix(fmt.Sprintf("daemon: cmd: %s", s))
}
