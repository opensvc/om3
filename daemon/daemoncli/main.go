package daemoncli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/soellman/pidfile"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/daemon"
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
		client *client.T
		node   string
	}
	waiter interface {
		Wait()
	}
)

func DaemonPidFile() string {
	return filepath.Join(rawconfig.Paths.Var, "osvcd.pid")
}

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

func (t *T) SetNode(node string) {
	t.node = node
}

// Start function will start daemon with internal lock protection
func (t *T) Start() error {
	release, err := getLock("Start")
	if err != nil {
		return err
	}
	daemonPidFile := DaemonPidFile()
	if err := pidfile.WriteControl(daemonPidFile, os.Getpid(), true); err != nil {
		return err
	}
	defer os.Remove(daemonPidFile)
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

// Stop function will stop daemon with internal lock protection
func (t *T) Stop() error {
	release, err := getLock("Stop")
	if err != nil {
		return err
	}
	defer release()
	return t.stop()
}

// ReStart function will restart daemon with internal lock protection
func (t *T) ReStart() error {
	release, err := getLock("Restart")
	if err != nil {
		return err
	}
	d, err := t.restart()
	release()
	if err != nil {
		return err
	}
	d.Wait()
	return nil
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

// LockFuncExit calls f() with cli lock protection
//
// os.exit(1) when lock failed or f() returns error
func LockFuncExit(desc string, f func() error) {
	if err := lock.Func(lockPath+"-cli", 60*time.Second, desc, f); err != nil {
		log.Logger.Error().Err(err).Msg(desc)
		os.Exit(1)
	}
}

// LockCmdExit starts cmd, then call checker() with cli lock protection
//
// os.exit(1) when lock failed or cmd.Start() or checker() returns error
func LockCmdExit(cmd *command.T, checker func() error, desc string) {
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
		os.Exit(1)
	}
}

// getLock() manage internal lock for functions that will stop/start/restart daemon
//
// It returns a release function to release lock
func getLock(desc string) (func(), error) {
	return lock.Lock(lockPath, lockTimeout, desc)
}

func (t *T) stop() error {
	log.Debug().Msg("cli-stop check running")
	if !t.running() {
		log.Debug().Msg("Already stopped")
		return nil
	}
	_, err := t.client.PostDaemonStop(context.Background())
	if err != nil {
		if !errors.Is(err, syscall.ECONNRESET) &&
			!strings.Contains(err.Error(), "unexpected EOF") &&
			!strings.Contains(err.Error(), "unexpected end of JSON input") {
			log.Debug().Err(err).Msgf("client.NewPostDaemonStop().Do(), error is %s", reflect.TypeOf(err))
			return err
		}
	}
	log.Debug().Msg("wait for stop...")
	if err := waitForBool(WaitStoppedTimeout, WaitStoppedDelay, true, t.notRunning); err != nil {
		log.Debug().Msg("cli-stop still running after stop")
		return errors.New("daemon still running after stop")
	}
	log.Debug().Msg("stopped")
	// one more delay before return listener not anymore responding
	time.Sleep(WaitStoppedDelay)
	return nil
}

func (t *T) start() (waiter, error) {
	if err := capabilities.Scan(); err != nil {
		return nil, err
	}
	log.Info().Strs("capabilities", capabilities.Data()).Msg("rescanned node capabilities")

	if err := rawconfig.CreateMandatoryDirectories(); err != nil {
		log.Error().Err(err).Msgf("cli-start can't create mandatory directories")
		return nil, err
	}
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

func (t *T) restart() (waiter, error) {
	if err := t.stop(); err != nil {
		return nil, err
	}
	d, err := t.start()
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (t *T) running() bool {
	resp, err := t.client.GetDaemonRunningWithResponse(context.Background())
	if err != nil {
		log.Debug().Err(err).Msg("daemon is not running")
		return false
	}
	nodename := t.node
	if nodename == "" {
		nodename = hostname.Hostname()
	}
	for _, item := range resp.JSON200.Data {
		if item.Endpoint == nodename {
			val := item.Data
			log.Debug().Msgf("daemon running is %v", val)
			return val
		}
	}
	log.Debug().Msgf("daemon is not running")
	return false
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
			return errors.New("timeout reached")
		case <-retryTicker.C:
			if f() == expected {
				return nil
			}
		}
	}
}
