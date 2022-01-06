package subdaemon

import (
	"errors"
)

// Start() will start the main daemon
// The manager daemon is responsible ot starting its sub daemons
func (t *T) Start() error {
	if t.Running() {
		t.log.Info().Msg("already started")
		return nil
	}
	return t.callAction("starting", "started")
}

// Stop() will stop the sub daemons, then the main sub daemon
func (t *T) Stop() error {
	if !t.Running() {
		t.log.Info().Msg("already stopped")
		return nil
	}
	return t.callAction("stopping", "stopped")
}

// Quit will stop the daemon routines
//
// no more register or action are then possible
func (t *T) actionsQuit() error {
	return t.callAction("quit", "done")
}

// ReStart() will stop sub daemon, then main daemon, then start the main daemon
//
// The main daemon is responsible ot starting its sub daemons
func (t *T) ReStart() error {
	return t.callAction("restarting", "restarted")
}

func (t *T) actions() error {
	if t.actionsEnabled() {
		return errors.New("call actions() on enabled")
	}
	t.mgrActionC = make(chan mgrAction)
	running := make(chan bool)
	go func() {
		defer t.Trace(t.Name() + "-actions")()
		t.mgrActionEnable.Enable()
		running <- true
		for {
			select {
			case a := <-t.mgrActionC:
				switch a.do {
				case "quit":
					t.log.Debug().Msg("actions quit")
					t.mgrActionEnable.Disable()
					a.done <- "done"
					return
				case "starting":
					t.log.Debug().Msg("actions start")
					if !t.Running() {
						<-t.start()
					}
					a.done <- "started"
					t.running.Enable()
				case "restarting":
					t.log.Debug().Msg("actions restart")
					if t.Running() {
						<-t.stop()
					}
					<-t.start()
					a.done <- "restarted"
					t.running.Enable()
				case "stopping":
					t.log.Debug().Msg("actions stop")
					if t.Running() {
						<-t.stop()
					}
					a.done <- "stopped"
					t.running.Disable()
				}
			}
		}
	}()
	<-running
	return nil
}

func (t *T) actionsEnabled() bool {
	return t.mgrActionEnable.Enabled()
}

func (t *T) stop() (done chan string) {
	t.log.Debug().Msg("stopping subs")
	done = make(chan string)
	subToWait := 0
	subDone := make(chan string)
	for sub := range t.subs() {
		subToWait = subToWait + 1
		sub := sub
		name := sub.Name()
		go func() {
			defer t.Trace(t.Name() + "-stopping-sub")()
			t.log.Debug().Msgf("stopping sub %s", name)
			if err := sub.Stop(); err != nil {
				t.log.Error().Err(err).Msgf("stop sub %s failed", name)
			}
			if err := sub.Quit(); err != nil {
				t.log.Error().Err(err).Msgf("quit %s failed", name)
			}
			t.log.Info().Msgf("stop sub %s done", name)
			if err := t.UnRegister(sub); err != nil {
				t.log.Error().Err(err).Msgf("UnRegister %s failed", name)
			}
			subDone <- name
		}()
	}
	go func() {
		defer t.Trace(t.Name() + "-stopping-sub-wait")()
		if subToWait > 0 {
			t.log.Info().Msgf("waiting for %d sub managers", subToWait)
			for i := 1; i <= subToWait; i++ {
				t.log.Debug().Msgf("waiting %d of %d", i, subToWait)
				<-subDone
				t.log.Debug().Msgf("done %d of %d", i, subToWait)
			}
		}
		if err := t.main.MainStop(); err != nil {
			t.log.Error().Err(err).Msg("MainStop failed")
		}
		done <- "stopped"
	}()

	return done
}

func (t *T) start() (done chan string) {
	done = make(chan string)
	go func() {
		defer t.Trace(t.Name() + "-MainStart")()
		if err := t.main.MainStart(); err != nil {
			t.log.Error().Err(err).Msg("start failed")
		}
		done <- "started"
	}()
	return done
}

func (t *T) callAction(do, wanted string) error {
	t.log.Info().Msg(do)
	if !t.actionsEnabled() {
		err := errors.New("callAction " + do + " on disabled action main")
		t.log.Error().Err(err).Msg("callAction")
		return err
	}
	resChan := make(chan string)
	t.mgrActionC <- mgrAction{do, resChan}
	t.log.Debug().Msgf("waiting %s", wanted)
	if result := <-resChan; result != wanted {
		t.log.Error().Msgf("%s failed got %s instead of %s", do, result, wanted)
		return errors.New(result)
	}
	t.log.Info().Msg(wanted)
	return nil
}
