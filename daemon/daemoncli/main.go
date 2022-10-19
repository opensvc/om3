package daemoncli

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/daemon/daemon"
	"opensvc.com/opensvc/daemon/daemonapi"
	"opensvc.com/opensvc/util/command"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/lock"
)

var (
	clientOptions      []funcopt.O
	lockPath           = "/tmp/locks/main"
	lockTimeout        = 60 * time.Second
	WaitRunningTimeout = 4 * time.Second
	WaitRunningDelay   = 100 * time.Millisecond
	WaitStoppedTimeout = 4 * time.Second
	WaitStoppedDelay   = 100 * time.Millisecond
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
	_, err := t.client.NewPostDaemonStop().Do()
	if err != nil &&
		!strings.Contains(err.Error(), "unexpected EOF") &&
		!strings.Contains(err.Error(), "unexpected end of JSON input") {
		return err
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
	request := t.client.NewGetDaemonRunning()
	request.SetNode(t.node)
	b, err := request.Do()
	if err != nil {
		log.Debug().Err(err).Msg("daemon is not running")
		return false
	}
	var resp daemonapi.ResponseMuxBool
	if err := json.Unmarshal(b, &resp.Data); err != nil {
		log.Error().Err(err).Msgf("Unmarshal b: %s", b)
		return false
	}
	nodename := t.node
	if nodename == "" {
		nodename = hostname.Hostname()
	}
	for _, item := range resp.Data {
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
	t := time.NewTimer(timeout)
	defer func() {
		if !t.Stop() {
			<-t.C
		}
	}()
	for {
		select {
		case <-t.C:
			return errors.New("timeout reached")
		default:
			if f() == expected {
				return nil
			}
			time.Sleep(retryDelay)
		}
	}
}
