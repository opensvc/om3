/*
	    Package subDaemon provides main and sub daemon management features

		Start, Stop, Restart

		1 go routines is used to serialize Start/Stop/Restart

		A subdaemon can have subdaemons
*/
package subdaemon

import (
	"context"
	"sync"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/daemon/enable"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/hostname"
)

type (
	T struct {
		sync.WaitGroup
		ctx         context.Context
		cancel      context.CancelFunc
		name        string
		log         zerolog.Logger
		children    []Manager
		main        Manager
		controlChan chan controlAction
		enabled     *enable.T
		running     *enable.T
	}

	controlAction struct {
		name string
		done chan error
	}
)

func (t *T) Log() zerolog.Logger {
	return t.log
}

func (t *T) Name() string {
	if t == nil {
		return ""
	}
	return t.name
}

// Enabled() returns true is daemon control actions are handled
func (t T) Enabled() bool {
	return t.enabled.Enabled()
}

// no more register or control action are then possible
func (t *T) disable() {
	t.log.Debug().Msg("disable")
	t.enabled.Disable()
}

// Running() returns true when MainManager daemon has been started
func (t *T) Running() bool {
	return t.running.Enabled()
}

func New(opts ...funcopt.O) *T {
	t := &T{
		enabled:     enable.New(),
		running:     enable.New(),
		controlChan: make(chan controlAction),
		children:    make([]Manager, 0),
	}
	if err := funcopt.Apply(t, opts...); err != nil {
		t.log.Error().Err(err).Msg("subdaemon funcopt.Apply")
		return nil
	}
	t.log = log.Logger.With().Str("sub", t.name).Str("n", hostname.Hostname()).Logger()
	return t
}
