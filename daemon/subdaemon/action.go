package subdaemon

import (
	"context"
	"sort"

	"github.com/pkg/errors"
)

// Start starts the control loop, the main loop and sub daemons
func (t *T) Start(ctx context.Context) error {
	t.ctx, t.cancel = context.WithCancel(ctx)
	if err := t.startControl(); err != nil {
		return err
	}
	if err := t.do("start"); err != nil {
		return err
	}
	return nil
}

// Stop stops the sub daemons, then the main loop, and the control loop
func (t *T) Stop() error {
	if !t.enabled.Enabled() {
		t.log.Debug().Msg("already stopped")
		return nil
	}
	if err := t.do("stop"); err != nil {
		t.log.Error().Err(err).Msg("stop")
		return err
	}
	t.Wait()
	return nil
}

// Restart chains stop and start
func (t *T) Restart(ctx context.Context) error {
	if err := t.Stop(); err != nil {
		return err
	}
	t.log.Debug().Msg("restart reached the bottom")
	if err := t.Start(ctx); err != nil {
		return err
	}
	return nil
}

func (t *T) startControl() error {
	if t.enabled.Enabled() {
		// for ex: restart, start-start
		return nil
	}
	running := make(chan bool)
	t.Add(1)
	go func() {
		defer t.Done()
		defer t.Trace(t.Name() + "-control")()
		running <- true
		t.controlLoop()
	}()
	<-running
	t.log.Debug().Msg("enable control")
	t.enabled.Enable()
	return nil
}

func (t *T) controlLoop() {
	for {
		select {
		case <-t.ctx.Done():
			// stop receiving new commands, but don't exit the loop
			// so the stop handler exits with a last result.
			t.enabled.Disable()
		case a := <-t.controlChan:
			switch a.name {
			case "start":
				a.done <- t.start()
			case "stop":
				if err := t.stop(); err != nil {
					a.done <- err
					return
				}
				t.disable()
				a.done <- nil
				return // exit control routine
			default:
				a.done <- errors.Errorf("unknown action: %s", a.name)
			}
		}
	}
}

// call via do() only for serialization
func (t *T) stop() error {
	if !t.Running() {
		t.log.Debug().Msgf("already stopped")
		return nil
	}
	newChildren := make([]Manager, 0)
	for i := len(t.children) - 1; i >= 0; i -= 1 {
		sub := t.children[i]
		name := sub.Name()
		if err := sub.Stop(); err != nil {
			// prevent sub unregistering
			newChildren = append(newChildren, sub)
			t.log.Error().Err(err).Msgf("stop %s failed", name)
		}
	}
	sort.SliceStable(newChildren, func(i, j int) bool {
		return i > j
	})
	t.children = newChildren
	if err := t.main.MainStop(); err != nil {
		t.log.Error().Err(err).Msg("main stop failed")
		return err
	}
	t.running.Disable()
	t.cancel() // exits the control loop
	t.log.Info().Msgf("stopped children and main")
	return nil
}

// call via do() only for serialization
func (t *T) start() error {
	if t.Running() {
		t.log.Debug().Msg("already started")
		return nil
	}
	t.log.Debug().Msg("start")
	if err := t.main.MainStart(t.ctx); err != nil {
		t.log.Error().Err(err).Msg("start")
		return err
	}
	t.running.Enable()
	t.log.Info().Msgf("started")
	return nil
}

// call via do() only for serialization
func (t *T) Register(sub Manager) error {
	t.children = append(t.children, sub)
	return nil
}

// do is a synchronous controlAction submitter
func (t *T) do(what string) error {
	if !t.enabled.Enabled() {
		err := errors.Errorf("disabled sub")
		t.log.Error().Err(err).Msgf("%s", what)
		return err
	}
	t.log.Debug().Msgf("queue %s", what)
	resChan := make(chan error)
	t.controlChan <- controlAction{what, resChan}
	if err := <-resChan; err != nil {
		t.log.Error().Err(err).Msgf("%s", what)
		return err
	}
	return nil
}
