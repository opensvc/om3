package daemoncli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/daemon"
	"github.com/opensvc/om3/daemon/daemonsys"
	"github.com/opensvc/om3/util/capabilities"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/lock"
)

var (
	lockPath           = "/tmp/locks/main"
	lockTimeout        = 60 * time.Second
	WaitRunningTimeout = 20 * time.Second
	WaitRunningDelay   = 500 * time.Millisecond
	WaitStoppedTimeout = 4 * time.Second
	WaitStoppedDelay   = 250 * time.Millisecond
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
		if err := ccfg.Config().Set(*op); err != nil {
			return err
		}
		if k.Obfuscate {
			op.Value = "xxxx"
		}
		log.Info().Msgf("bootstrap cluster config: %s", op)
	}

	if err := ccfg.Config().Commit(); err != nil {
		return err
	}
	rawconfig.LoadSections()
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
func (t *T) RestartFromCmd(ctx context.Context, foreground bool) error {
	if t.daemonsys == nil {
		log.Info().Msg("daemon restart (origin os)")
		return t.restartFromCmd(foreground)
	}
	if foreground {
		return t.restartFromCmd(foreground)
	}
	defer func() {
		_ = t.daemonsys.Close()
	}()
	if ok, err := t.daemonsys.Defined(ctx); err != nil || !ok {
		log.Info().Msg("daemon restart (origin os, no unit defined)")
		return t.restartFromCmd(foreground)
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
	if err := rawconfig.CreateMandatoryDirectories(); err != nil {
		log.Error().Err(err).Msgf("cli-start can't create mandatory directories")
		return err
	}
	release, err := getLock("Start")
	if err != nil {
		return err
	}

	d, err := t.start()
	release()
	if err != nil {
		return err
	}
	if d != nil {
		d.Wait()
	}
	return nil
}

// StartFromCmd handle daemon start from command origin.
//
// It is used to forward start control to (systemd) manager (when the origin is not systemd)
func (t *T) StartFromCmd(ctx context.Context, foreground bool, profile string) error {
	if t.daemonsys == nil {
		log.Info().Msg("daemon start (origin os)")
		return t.startFromCmd(foreground, profile)
	}
	defer func() {
		_ = t.daemonsys.Close()
	}()
	if ok, err := t.daemonsys.Defined(ctx); err != nil || !ok {
		log.Info().Msg("daemon start (origin os, no unit defined)")
		return t.startFromCmd(foreground, profile)
	}
	if t.daemonsys.CalledFromManager() {
		if foreground {
			log.Info().Msg("daemon start foreground (origin manager)")
			return t.startFromCmd(foreground, profile)
		}
		if t.Running() {
			log.Info().Msg("daemon start is already running (origin manager)")
			return nil
		}
		log.Info().Msg("daemon start run new cmd --foreground (origin manager)")
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
		log.Info().Msg("daemon start foreground (origin os)")
		return t.startFromCmd(foreground, profile)
	} else {
		log.Info().Msg("daemon start forward to manager (origin os)")
		return t.managerStart(ctx)
	}
}

// StopFromCmd handle daemon stop from command origin.
//
// It is used to forward stop control to (systemd) manager (when the origin is not systemd)
func (t *T) StopFromCmd(ctx context.Context) error {
	if t.daemonsys == nil {
		log.Info().Msg("daemon stop (origin os)")
		return t.Stop()
	}
	defer func() {
		_ = t.daemonsys.Close()
	}()
	if ok, err := t.daemonsys.Defined(ctx); err != nil || !ok {
		log.Info().Msg("daemon stop (origin os, no unit defined)")
		return t.Stop()
	}
	if t.daemonsys.CalledFromManager() {
		log.Info().Msg("daemon stop (origin manager)")
		return t.Stop()
	}
	log.Info().Msg("daemon stop forward to manager (origin os)")
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

// Running function detect daemon status using api
//
// it returns true is daemon is running, else false
func (t *T) Running() bool {
	return t.running()
}

// WaitRunning function waits for daemon running
//
// It needs to be called from a cli lock protection
func (t *T) WaitRunning() error {
	return waitForBool(WaitRunningTimeout, WaitRunningDelay, true, t.running)
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
			log.Logger.Error().Err(err).Msg("failed command: " + desc)
			return err
		}
		if checker != nil {
			if err := checker(); err != nil {
				log.Logger.Error().Err(err).Msg("failed checker: " + desc)
				return err
			}
		}
		return nil
	}
	if err := lock.Func(lockPath+"-cli", 60*time.Second, desc, f); err != nil {
		log.Logger.Error().Err(err).Msg(desc)
		return err
	}
	return nil
}

func (t *T) managerRestart() error {
	name := "forward restart daemon to manager"
	log.Info().Msgf("%s...", name)
	if err := t.daemonsys.Restart(); err != nil {
		return fmt.Errorf("%s failed: %w", name, err)
	}
	return nil
}

func (t *T) managerStart(ctx context.Context) error {
	name := "forward start daemon to manager"
	log.Info().Msgf("%s...", name)
	if err := t.daemonsys.Start(ctx); err != nil {
		return fmt.Errorf("%s failed: %w", name, err)
	}
	if err := t.WaitRunning(); err != nil {
		return fmt.Errorf("%s failed during wait running: %w", name, err)
	}
	return nil
}

func (t *T) managerStop(ctx context.Context) error {
	name := "forward stop daemon to manager"
	log.Info().Msgf("%s...", name)
	if ok, err := t.daemonsys.Activated(ctx); err != nil {
		err := fmt.Errorf("%s can't detect activated state: %w", name, err)
		return err
	} else if !ok && t.Running() {
		// recover inconsistent manager view not activated, but reality is running
		if err := t.Stop(); err != nil {
			return fmt.Errorf("%s failed during recover: %w", name, err)
		}
	}
	if err := t.daemonsys.Stop(ctx); err != nil {
		return fmt.Errorf("%s failed during stop: %w", name, err)
	}
	return nil
}

func (t *T) restartFromCmd(foreground bool) error {
	if err := t.Stop(); err != nil {
		return err
	}
	return t.startFromCmd(foreground, "")
}

func (t *T) stop() error {
	log.Debug().Msg("cli-stop check running")
	if !t.running() {
		log.Debug().Msg("Already stopped")
		return nil
	}
	resp, err := t.client.PostDaemonStop(context.Background())
	if err != nil {
		if !errors.Is(err, syscall.ECONNRESET) &&
			!strings.Contains(err.Error(), "unexpected EOF") &&
			!strings.Contains(err.Error(), "unexpected end of JSON input") {
			log.Debug().Err(err).Msgf("client.NewPostDaemonStop().Do(), error is %s", reflect.TypeOf(err))
			return err
		}
	}
	switch resp.StatusCode {
	case 200:
		log.Debug().Msg("wait for stop...")
		if err := waitForBool(WaitStoppedTimeout, WaitStoppedDelay, true, t.notRunning); err != nil {
			log.Debug().Msg("cli-stop still running after stop")
			return fmt.Errorf("daemon still running after stop")
		}
		log.Debug().Msg("stopped")
		// one more delay before return listener not anymore responding
		time.Sleep(WaitStoppedDelay)
	default:
		return fmt.Errorf("unexpected status code: %s", resp.Status)
	}
	return nil
}

func (t *T) start() (waiter, error) {
	if err := capabilities.Scan(); err != nil {
		return nil, err
	}
	log.Info().Strs("capabilities", capabilities.Data()).Msg("rescanned node capabilities")

	if err := bootStrapCcfg(); err != nil {
		return nil, err
	}
	log.Debug().Msg("cli-start check if not already running")
	if t.running() {
		log.Debug().Msg("Already started")
		return nil, nil
	}
	log.Debug().Msg("cli-start RunDaemon")
	d, err := daemon.RunDaemon()
	if err != nil {
		return nil, err
	}
	log.Debug().Msg("cli-start daemon started")
	return d, nil
}

func (t *T) startFromCmd(foreground bool, profile string) error {
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
		if err := t.Start(); err != nil {
			return fmt.Errorf("start daemon cli: %w", err)
		}
		return nil
	} else {
		checker := func() error {
			if err := t.WaitRunning(); err != nil {
				err := fmt.Errorf("start checker wait running failed: %w", err)
				log.Error().Err(err).Msg("starting daemon")
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

func (t *T) running() bool {
	resp, err := t.client.GetDaemonRunningWithResponse(context.Background())
	if err != nil {
		log.Debug().Err(err).Msg("daemon is not running")
		return false
	} else if resp.StatusCode() != http.StatusOK {
		log.Warn().Msgf("unexpected get daemon running status code %s", resp.Status())
		return false
	}
	log.Debug().Msgf("daemon running is %v", *resp.JSON200)
	return *resp.JSON200
}

func (t *T) notRunning() bool {
	return !t.running()
}

func waitForBool(timeout, retryDelay time.Duration, expected bool, f func() bool) error {
	retryTicker := time.NewTicker(retryDelay)
	defer retryTicker.Stop()

	timeoutTicker := time.NewTicker(timeout)
	defer timeoutTicker.Stop()

	for {
		select {
		case <-timeoutTicker.C:
			return fmt.Errorf("timeout reached")
		case <-retryTicker.C:
			if f() == expected {
				return nil
			}
		}
	}
}
