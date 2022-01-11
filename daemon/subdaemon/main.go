/*
    Package subDaemon provides main and sub daemon management features

	Start, Stop, Init, Quit, Restart

	2 go routines are used:
        reg routine to manage registration of sub daemons (under responsability of this subdaemon)
        actions routine that manage Start/Stop/Restart

	a sub daemon can also manage other daemons (that can also contain some other sub daemons)
*/
package subdaemon

import (
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"opensvc.com/opensvc/daemon/enable"
	"opensvc.com/opensvc/daemon/routinehelper"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/logging"
	"opensvc.com/opensvc/util/xsession"
)

type (
	T struct {
		name            string
		log             zerolog.Logger
		logName         string
		subSvc          map[string]MainManager
		main            MainManager
		mgrActionC      chan mgrAction
		mgrActionEnable *enable.T
		regActionC      chan registerAction
		regActionEnable *enable.T
		enabled         *enable.T
		running         *enable.T
		done            chan bool
		routinehelper.TT
	}

	registerAction struct {
		action   string
		managerC chan MainManager
		done     chan string
	}

	mgrAction struct {
		do   string
		done chan string
	}

	MainManager interface {
		Name() string
		Stop() error
		MainStart() error
		MainStop() error
		Running() bool
		Quit() error
		WaitDone()
	}
)

func (t *T) Log() zerolog.Logger {
	return t.log
}

func (t *T) Name() string {
	return t.name
}

// Enabled() returns true is daemon is enabled
//
// It accecpts registration of other subdaemons
// It accecpts actions Stop/Start/Quit
func (t *T) Enabled() bool {
	return t.enabled.Enabled()
}

// Running() returns true when MainManager daemon has been started
func (t *T) Running() bool {
	return t.running.Enabled()
}

// Init() will start daemon routine management
//
// o register routine to allow register sub daemons
// o action to allow action on daemon
func (t *T) Init() error {
	if t.Enabled() {
		err := errors.New("call Init on already initialized")
		t.log.Error().Err(err).Msg("Init failed")
		return err
	}
	t.done = make(chan bool)
	if err := t.subRegister(); err != nil {
		t.log.Error().Err(err).Msg("Init")
		return err
	}
	if err := t.actions(); err != nil {
		t.log.Error().Err(err).Msg("Init")
		return err
	}
	t.enabled.Enable()
	return nil
}

func (t *T) WaitDone() {
	t.log.Debug().Msg("WaitDone for Daemon ended")
	<-t.done
	t.log.Info().Msg("Daemon ended")
}

// Quit will stop the 2 daemon routines
// o reg routine
// o action routine
func (t *T) Quit() error {
	if !t.Enabled() {
		err := errors.New("call quit on already disabled")
		t.log.Error().Err(err).Msg("Quit")
		return err
	}
	if err := t.subRegisterQuit(); err != nil {
		t.log.Error().Err(err).Msg("subRegisterQuit")
		return err
	}
	if err := t.actionsQuit(); err != nil {
		t.log.Error().Err(err).Msg("actionsQuit")
		return err
	}
	t.enabled.Disable()
	t.done <- true
	return nil
}

func New(opts ...funcopt.O) *T {
	t := &T{
		regActionEnable: enable.New(),
		mgrActionEnable: enable.New(),
		enabled:         enable.New(),
		running:         enable.New(),
		logName:         "daemon",
	}
	t.SetTracer(routinehelper.NewTracerNoop())
	if err := funcopt.Apply(t, opts...); err != nil {
		t.log.Error().Err(err).Msg("subdaemon funcopt.Apply")
		return nil
	}
	t.log = logging.Configure(logging.Config{
		ConsoleLoggingEnabled: true,
		EncodeLogsAsJSON:      true,
		FileLoggingEnabled:    true,
		Directory:             "/tmp/log",
		Filename:              t.logName + ".log",
		MaxSize:               5,
		MaxBackups:            1,
		MaxAge:                30,
	}).
		With().
		Str("n", hostname.Hostname()).
		Str("sid", xsession.ID).
		Str("name", t.name).
		Logger()
	t.subSvc = make(map[string]MainManager)
	return t
}
