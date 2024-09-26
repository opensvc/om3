package daemoncmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clusterdump"
	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/daemon"
	"github.com/opensvc/om3/daemon/daemonsys"
	"github.com/opensvc/om3/util/capabilities"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/lock"
	"github.com/opensvc/om3/util/plog"
)

var (
	lockPath           = "/tmp/locks/main"
	lockTimeout        = 60 * time.Second
	WaitRunningTimeout = 20 * time.Second
	WaitRunningDelay   = 500 * time.Millisecond
	WaitStoppedTimeout = 4 * time.Second
	WaitStoppedDelay   = 250 * time.Millisecond
	errGoTest          = errors.New("running from go test")
)

type (
	T struct {
		client    *client.T
		node      string
		daemonsys Manager
	}
	waiter interface {
		Wait()
	}

	Manager interface {
		Activated(ctx context.Context) (bool, error)
		CalledFromManager() bool
		Close() error
		Defined(ctx context.Context) (bool, error)
		Start(ctx context.Context) error
		Restart() error
		Stop(context.Context) error
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
		log.Infof("bootstrap cluster config: %s", op)
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
		log.Infof("bootstrap cluster config to have at least one heartbeat: %s", op)
	}

	if err := ccfg.Config().Commit(); err != nil {
		return err
	}
	clusterdump.SetConfig()
	return nil
}

func New(c *client.T) *T {
	return &T{client: c}
}

func NewContext(ctx context.Context, c *client.T) *T {
	t := &T{client: c}
	var (
		i   interface{}
		err error
	)
	if i, err = daemonsys.New(ctx); err == nil {
		if mgr, ok := i.(Manager); ok {
			t.daemonsys = mgr
		}
	}
	return t
}

// RestartFromCmd handle daemon restart from command origin.
//
// It is used to forward restart control to (systemd) manager (when the origin is not systemd)
func (t *T) RestartFromCmd(ctx context.Context) error {
	log := logger("cli restart: ")
	if t.daemonsys == nil {
		log.Infof("origin os")
		return t.restartFromCmd()
	}
	defer func() {
		_ = t.daemonsys.Close()
	}()
	if ok, err := t.daemonsys.Defined(ctx); err != nil || !ok {
		log.Infof("origin os, no unit defined")
		return t.restartFromCmd()
	}
	// note: always ask manager for restart (during POST /daemon/restart handler
	// the server api is probably CalledFromManager). And systemd unit doesn't define
	// restart command.
	return t.managerRestart()
}

func (t *T) SetNode(node string) {
	t.node = node
}

// Start function will start daemon with internal lock protection
func (t *T) Start() error {
	log := logger("locked start: ")
	if err := rawconfig.CreateMandatoryDirectories(); err != nil {
		log.Errorf("can't create mandatory directories: %s", err)
		return err
	}
	release, err := getLock("Start")
	if err != nil {
		return err
	}
	isRunning, err := t.isRunning()
	if err != nil {
		return err
	}
	if isRunning {
		log.Infof("already started")
		return nil
	}
	pidFile := daemonPidFile()
	log.Debugf("create pid file %s", pidFile)

	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644); err != nil {
		return err
	}
	defer func() {
		log.Debugf("remove pid file %s", pidFile)
		if err := os.Remove(pidFile); err != nil {
			log.Errorf("remove pid file %s: %s", pidFile, err)
		}
	}()
	d, err := t.start()
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

// StartFromCmd handle daemon start from command origin.
//
// It is used to forward start control to (systemd) manager (when the origin is not systemd)
func (t *T) StartFromCmd(ctx context.Context, foreground bool, profile string) error {
	log := logger("cli start: ")
	if t.daemonsys == nil {
		log.Infof("origin os")
		return t.startFromCmd(foreground, profile)
	}
	defer func() {
		_ = t.daemonsys.Close()
	}()
	if ok, err := t.daemonsys.Defined(ctx); err != nil || !ok {
		log.Infof("origin os, no unit defined")
		return t.startFromCmd(foreground, profile)
	}
	if t.daemonsys.CalledFromManager() {
		if foreground {
			log.Infof("foreground (origin manager)")
			return t.startFromCmd(foreground, profile)
		}
		if isRunning, err := t.isRunning(); err != nil {
			return err
		} else if isRunning {
			log.Infof("already running (origin manager)")
			return nil
		}
		log.Infof("run new cmd --foreground (origin manager)")
		args := []string{"daemon", "start", "--foreground"}
		cmd := command.New(
			command.WithName(os.Args[0]),
			command.WithArgs(args),
		)
		checker := func() error {
			if err := t.WaitRunning(); err != nil {
				return fmt.Errorf("start checker wait running failed: %w", err)
			}
			return nil
		}
		return lockCmdCheck(cmd, checker, "daemon start")
	} else if foreground {
		log.Infof("foreground (origin os)")
		return t.startFromCmd(foreground, profile)
	} else {
		log.Infof("origin os")
		return t.managerStart(ctx)
	}
}

// StopFromCmd handle daemon stop from command origin.
//
// It is used to forward stop control to (systemd) manager (when the origin is not systemd)
func (t *T) StopFromCmd(ctx context.Context) error {
	log := logger("cli stop: ")
	if t.daemonsys == nil {
		log.Infof("origin os")
		return t.Stop()
	}
	defer func() {
		_ = t.daemonsys.Close()
	}()
	if ok, err := t.daemonsys.Defined(ctx); err != nil || !ok {
		log.Infof("origin os, no unit defined")
		return t.Stop()
	}
	if t.daemonsys.CalledFromManager() {
		log.Infof("origin manager")
		return t.Stop()
	}
	log.Infof("origin os")
	return t.managerStop(ctx)
}

// Stop function will stop daemon with internal lock protection
func (t *T) Stop() error {
	release, err := getLock("Stop")
	if err != nil {
		return err
	}
	defer release()
	return t.stop()
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
	return waitForBool(WaitRunningTimeout, WaitRunningDelay, true, t.isRunning)
}

// getLock() manage internal lock for functions that will stop/start/restart daemon
//
// It returns a release function to release lock
func getLock(desc string) (func(), error) {
	return lock.Lock(lockPath, lockTimeout, desc)
}

// lockCmdCheck starts cmd, then call checker() with cli lock protection
func lockCmdCheck(cmd *command.T, checker func() error, desc string) error {
	log := logger("lock cmd: ")
	f := func() error {
		if err := cmd.Start(); err != nil {
			log.Errorf("failed command: %s: %s", desc, err)
			return err
		}
		if checker != nil {
			if err := checker(); err != nil {
				log.Errorf("failed checker: %s: %s", desc, err)
				return err
			}
		}
		return nil
	}
	if err := lock.Func(lockPath+"-cli", 60*time.Second, desc, f); err != nil {
		log.Errorf("failed %s: %s", desc, err)
		return err
	}
	return nil
}

func (t *T) managerRestart() error {
	log := logger("restart with manager: ")
	log.Infof("forward to daemonsys...")
	name := "restart with manager"
	if err := t.daemonsys.Restart(); err != nil {
		return fmt.Errorf("%s: daemonsys restart failed: %w", name, err)
	}
	return nil
}

func (t *T) managerStart(ctx context.Context) error {
	log := logger("start with manager: ")
	log.Infof("forward to daemonsys...")
	name := "start with manager"
	if err := t.daemonsys.Start(ctx); err != nil {
		return fmt.Errorf("%s: daemonsys restart failed: %w", name, err)
	}
	if err := t.WaitRunning(); err != nil {
		return fmt.Errorf("%s: wait running failed: %w", name, err)
	}
	return nil
}

func (t *T) managerStop(ctx context.Context) error {
	log := logger("stop with manager: ")
	log.Infof("forward to daemonsys...")
	name := "stop with manager"
	if ok, err := t.daemonsys.Activated(ctx); err != nil {
		err := fmt.Errorf("%s: can't detect activated state: %w", name, err)
		return err
	} else if !ok {
		// recover inconsistent manager view not activated, but reality is running
		if err := t.Stop(); err != nil {
			return fmt.Errorf("%s: failed during recover: %w", name, err)
		}
	} else {
		if err := t.daemonsys.Stop(ctx); err != nil {
			return fmt.Errorf("%s: daemonsys stop: %w", name, err)
		}
	}

	return nil
}

func (t *T) restartFromCmd() error {
	if err := t.Stop(); err != nil {
		return err
	}
	return t.startFromCmd(false, "")
}

func (t *T) stop() error {
	log := logger("stop: ")
	resp, err := t.client.PostDaemonStopWithResponse(context.Background(), hostname.Hostname())
	if err != nil {
		if !errors.Is(err, syscall.ECONNRESET) &&
			!strings.Contains(err.Error(), "unexpected EOF") &&
			!strings.Contains(err.Error(), "unexpected end of JSON input") {
			log.Debugf("post daemon stp: %s... kill", err)
			return t.kill()
		}
		return err
	}
	switch {
	case resp.JSON200 != nil:
		log.Debugf("wait for stop...")
		pid := resp.JSON200.Pid
		fn := func() (bool, error) {
			return t.isNotRunning(pid)
		}
		if err := waitForBool(WaitStoppedTimeout, WaitStoppedDelay, true, fn); err != nil {
			log.Debugf("daemon %d still running after stop: %s ... kill", pid, err)
			return t.kill()
		}
		log.Debugf("stopped")
		// one more delay before return listener not anymore responding
		time.Sleep(WaitStoppedDelay)
	default:
		log.Debugf("unexpected status code: %s... kill", resp.Status())
		return t.kill()
	}

	return nil
}

func (t *T) start() (*daemon.T, error) {
	log := logger("start: ")
	if err := capabilities.Scan(); err != nil {
		return nil, err
	}
	log.Attr("capabilities", capabilities.Data()).Infof("rescanned node capabilities")

	if err := bootStrapCcfg(); err != nil {
		return nil, err
	}
	d := daemon.New()
	log.Debugf("starting daemon...")
	return d, d.Start(context.Background())
}

func (t *T) startFromCmd(foreground bool, profile string) error {
	log := logger("start from cmd: ")
	if foreground {
		if profile != "" {
			f, err := os.Create(profile)
			if err != nil {
				return fmt.Errorf("create CPU profile: %w", err)
			}
			defer func() {
				_ = f.Close()
			}()
			if err := pprof.StartCPUProfile(f); err != nil {
				return fmt.Errorf("start CPU profile: %w", err)
			}
			defer pprof.StopCPUProfile()
		}
		if t.daemonsys != nil {
			if err := t.daemonsys.Close(); err != nil {
				return fmt.Errorf("unable to close daemonsys: %w", err)
			}
		}
		if err := t.Start(); err != nil {
			return fmt.Errorf("start daemon: %w", err)
		}
		return nil
	} else {
		checker := func() error {
			if err := t.WaitRunning(); err != nil {
				err := fmt.Errorf("start checker wait running failed: %w", err)
				log.Errorf("wait running: %s", err)
				return err
			}
			return nil
		}
		args := []string{"daemon", "start", "--foreground"}
		if t.daemonsys == nil {
			args = append(args, "--native")
		}
		cmd := command.New(
			command.WithName(os.Args[0]),
			command.WithArgs(args),
		)
		return lockCmdCheck(cmd, checker, "daemon start")
	}
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

func (t *T) isNotRunning(pid int) (bool, error) {
	_, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return false, nil
}

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

	isNotRunning, err := t.isNotRunning(pid)
	if err != nil {
		return false, err
	}
	return !isNotRunning, err
}

func (t *T) getPid() (int, error) {
	pidFile := daemonPidFile()
	pid, err := extractPidFromPidFile(pidFile)
	if errors.Is(err, os.ErrNotExist) {
		return -1, nil
	}
	if err != nil {
		return -1, err
	}
	v, err := isCmdlineMatchingDaemon(pid)
	if !v {
		return -1, err
	}
	return pid, err
}

func isCmdlineMatchingDaemon(pid int) (bool, error) {
	log := logger("test:")
	log.Debugf("test cmdline")
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))

	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	if strings.Contains(string(b), "/daemoncmd.test") {
		return true, errGoTest
	}

	sep := make([]byte, 1)
	l := bytes.Split(b, sep)

	if string(l[1]) != "daemon" || string(l[2]) != "start" {
		return false, fmt.Errorf("process %d pointed by %s is not a om daemon", pid, daemonPidFile())
	}
	return true, nil

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

func waitForBool(timeout, retryDelay time.Duration, expected bool, f func() (bool, error)) error {
	retryTicker := time.NewTicker(retryDelay)
	defer retryTicker.Stop()

	timeoutTicker := time.NewTicker(timeout)
	defer timeoutTicker.Stop()

	for {
		select {
		case <-timeoutTicker.C:
			return fmt.Errorf("timeout reached")
		case <-retryTicker.C:
			v, err := f()
			if err != nil {
				return err
			}
			if v == expected {
				return nil
			}
		}
	}
}

func logger(s string) *plog.Logger {
	return plog.NewDefaultLogger().
		Attr("pkg", "daemon/daemoncmd").
		WithPrefix(fmt.Sprintf("daemon: cmd: %s", s))
}
