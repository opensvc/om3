package monitor

import (
	"time"

	"github.com/rs/zerolog"

	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/daemondatactx"
	"opensvc.com/opensvc/daemon/enable"
	"opensvc.com/opensvc/daemon/routinehelper"
	"opensvc.com/opensvc/daemon/subdaemon"
	"opensvc.com/opensvc/util/funcopt"
)

type (
	T struct {
		*subdaemon.T
		daemonctx.TCtx
		log          zerolog.Logger
		loopC        chan action
		loopDelay    time.Duration
		loopEnabled  *enable.T
		routineTrace routineTracer
		routinehelper.TT
	}
	action struct {
		do   string
		done chan string
	}
	routineTracer interface {
		Trace(string) func()
		Stats() routinehelper.Stat
	}
)

func New(opts ...funcopt.O) *T {
	t := &T{
		TCtx:        daemonctx.TCtx{},
		loopDelay:   5 * time.Second,
		loopEnabled: enable.New(),
	}
	t.SetTracer(routinehelper.NewTracerNoop())
	if err := funcopt.Apply(t, opts...); err != nil {
		t.log.Error().Err(err).Msg("monitor funcopt.Apply")
		return nil
	}
	t.T = subdaemon.New(
		subdaemon.WithName("monitor"),
		subdaemon.WithMainManager(t),
		subdaemon.WithRoutineTracer(&t.TT),
	)
	t.log = t.Log()
	t.loopC = make(chan action)
	return t
}

func (t *T) MainStart() error {
	t.log.Info().Msg("mgr starting")
	started := make(chan bool)
	go func() {
		defer t.Trace(t.Name() + "-loop")()
		t.loop(started)
	}()
	<-started
	t.log.Info().Msg("mgr started")
	return nil
}

func (t *T) MainStop() error {
	t.log.Info().Msg("mgr stopping")
	if t.loopEnabled.Enabled() {
		done := make(chan string)
		t.loopC <- action{"stop", done}
		<-done
	}
	t.log.Info().Msg("mgr stopped")
	return nil
}

func (t *T) loop(c chan bool) {
	t.log.Info().Msg("loop started")
	t.loopEnabled.Enable()
	t.aLoop()
	c <- true
	for {
		select {
		case a := <-t.loopC:
			t.loopEnabled.Disable()
			t.log.Info().Msg("loop stopped")
			a.done <- "loop stopped"
			return
		case <-time.After(t.loopDelay):
			t.aLoop()
		}
	}
}

var (
	// For demo
	demoAvails = map[string]string{
		"dev1n1":        "",
		"dev1n2":        "",
		"dev1n3":        "",
		"u2004-local-1": "",
		"u2004-local-2": "",
		"u2004-local-3": "",
	}
	demoSvc = "demo"
)

func (t *T) aLoop() {
	t.log.Debug().Msg("loop")
	// For demo
	dataCmd := daemondatactx.DaemonData(t.Ctx)
	dataCmd.CommitPending()
	status := dataCmd.GetStatus()
	for remote, v := range demoAvails {
		remoteNodeStatus := daemondata.GetNodeStatus(status, remote)
		if remoteNodeStatus != nil {
			if demoStatus, ok := remoteNodeStatus.Services.Status[demoSvc]; ok {
				if v != demoStatus.Avail.String() {
					t.log.Info().Msgf("%s@%s status changed from %s -> %s", demoSvc, remote, v, demoStatus.Avail.String())
					demoAvails[remote] = demoStatus.Avail.String()
				}
			}
		}
	}
	//nodeData := dataCmd.GetLocalNodeStatus()
	//localhost := hostname.Hostname()
	//gen := nodeData.Gen[localhost]
	//nodeData.Gen[localhost] = gen + 1
	//dataCmd.ApplyFull(localhost, nodeData)
	//dataCmd.CommitPending()
}
