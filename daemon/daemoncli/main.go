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
	"opensvc.com/opensvc/daemon/daemon"
	"opensvc.com/opensvc/daemon/daemonenv"
	"opensvc.com/opensvc/util/command"
	"opensvc.com/opensvc/util/funcopt"
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
	waitDowner interface {
		WaitDone()
	}
)

func init() {
	var proto int
	proto = time.Now().Second() % 3
	switch proto {
	case 0:
		clientOptions = append(clientOptions, client.WithURL(daemonenv.UrlUxRaw))
	case 1:
		clientOptions = append(clientOptions, client.WithURL(daemonenv.UrlUxHttp))
	case 2:
		clientOptions = append(
			clientOptions,
			client.WithURL(daemonenv.UrlInetHttp))

		clientOptions = append(clientOptions,
			client.WithInsecureSkipVerify())

		clientOptions = append(clientOptions,
			client.WithCertificate(daemonenv.CertFile))

		clientOptions = append(clientOptions,

			client.WithKey(daemonenv.KeyFile),
		)
	}
}

// Start function will start daemon with internal lock protection
func Start() error {
	release, err := getLock("Start")
	if err != nil {
		return err
	}
	d, err := start()
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
func Stop() error {
	release, err := getLock("Stop")
	if err != nil {
		return err
	}
	defer release()
	return stop()
}

// ReStart function will restart daemon with internal lock protection
func ReStart() error {
	release, err := getLock("Restart")
	if err != nil {
		return err
	}
	d, err := restart()
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
func Running() bool {
	return running()
}

// WaitRunning function waits for daemon running
//
// It needs to be called from a cli lock protection
func WaitRunning() error {
	return waitForBool(WaitRunningTimeout, WaitRunningDelay, true, running)
}

// Events function is a cli for daemon/eventsdemo
func Events() error {
	if !running() {
		log.Debug().Msg("not running")
		return nil
	}
	cli, err := client.New(clientOptions...)
	if err != nil {
		return err
	}
	eventC, err := cli.NewGetEventsDemo().Do()
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

func stop() error {
	log.Debug().Msg("cli-stop check running")
	if !running() {
		log.Debug().Msg("Already stopped")
		return nil
	}
	cli, err := client.New(clientOptions...)
	if err != nil {
		return err
	}
	_, err = cli.NewPostDaemonStop().Do()
	if err != nil && !strings.Contains(err.Error(), "unexpected EOF") {
		return err
	}
	if running() {
		log.Debug().Msg("cli-stop still running after stop")
		return errors.New("daemon still running after stop")
	}
	return nil
}

func start() (waitDowner, error) {
	log.Debug().Msg("cli-start check if not already running")
	if running() {
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

func restart() (waitDowner, error) {
	if err := stop(); err != nil {
		return nil, err
	}
	d, err := start()
	if err != nil {
		return nil, err
	}
	return d, nil
}

func running() bool {
	var data []byte
	cli, err := client.New(clientOptions...)
	if err != nil {
		log.Error().Err(err).Msg("Running client.New")
		return false
	}
	data, err = cli.NewGetDaemonRunning().Do()
	if err != nil || string(data) != "running" {
		return false
	}
	running := string(data)
	log.Debug().Msgf("Running is %s", string(data))
	if running == "running" {
		return true
	}
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
