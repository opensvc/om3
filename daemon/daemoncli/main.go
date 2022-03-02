package daemoncli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/client/api"
	"opensvc.com/opensvc/daemon/daemon"
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
)

type (
	T struct {
		client *client.T
		node   string
	}
	waitDowner interface {
		WaitDone()
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
		d.WaitDone()
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
	d.WaitDone()
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

// Events function is a cli for daemon/eventsdemo
func (t *T) Events() error {
	if !t.running() {
		log.Debug().Msg("not running")
		return nil
	}
	eventC, err := t.client.NewGetEventsDemo().Do()
	if err != nil {
		return err
	}
	for ev := range eventC {
		log.Debug().Msgf("Events receive ev: %#v", ev)
		if b, err := json.MarshalIndent(ev, "", "  "); err != nil {
			return err
		} else {
			fmt.Printf("%s\n", b)
		}
	}
	return nil
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
	log.Debug().Msg("Check if still running")
	if t.running() {
		log.Debug().Msg("cli-stop still running after stop")
		return errors.New("daemon still running after stop")
	}
	log.Debug().Msg("Check if still running done")
	return nil
}

func (t *T) start() (waitDowner, error) {
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

func (t *T) restart() (waitDowner, error) {
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
	var nodesData api.GetDaemonRunningData
	if err := json.Unmarshal(b, &nodesData); err != nil {
		log.Error().Err(err).Msgf("Unmarshal b: %s", b)
		return false
	}
	nodename := t.node
	if nodename == "" {
		nodename = hostname.Hostname()
	}
	for _, item := range nodesData {
		if item.Endpoint == nodename {
			val := item.Data
			log.Debug().Msgf("daemon running is %v", val)
			return val
		}
	}
	log.Debug().Msgf("daemon is not running")
	return false
}

func waitForBool(timeout, retryDelay time.Duration, expected bool, f func() bool) error {
	max := time.After(timeout)
	for {
		select {
		case <-max:
			return errors.New("timeout reached")
		default:
			if f() == expected {
				return nil
			}
			<-time.After(retryDelay)
		}
	}
}
