package monitor

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/enable"
	"opensvc.com/opensvc/daemon/hbcache"
	"opensvc.com/opensvc/daemon/routinehelper"
	"opensvc.com/opensvc/daemon/subdaemon"
	"opensvc.com/opensvc/util/funcopt"
)

type (
	T struct {
		*subdaemon.T
		routinehelper.TT
		ctx          context.Context
		cancel       context.CancelFunc
		log          zerolog.Logger
		loopC        chan action
		loopDelay    time.Duration
		loopEnabled  *enable.T
		routineTrace routineTracer
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
		loopDelay:   1000 * time.Millisecond,
		loopEnabled: enable.New(),
		log:         log.Logger.With().Str("name", "monitor").Logger(),
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
	t.loopC = make(chan action)
	return t
}

func (t *T) MainStart(ctx context.Context) error {
	t.ctx, t.cancel = context.WithCancel(ctx)
	started := make(chan error)
	t.Add(1)
	go func() {
		defer t.Done()
		defer t.Trace(t.Name() + "-loop")()
		started <- nil
		t.loop()
	}()
	<-started
	return nil
}

func (t *T) MainStop() error {
	t.cancel()
	return nil
}

func (t *T) loop() {
	t.log.Info().Msg("loop started")
	ticker := time.NewTicker(t.loopDelay)
	defer ticker.Stop()
	sentGens := make(map[string]uint64)
	daemonData := daemondata.FromContext(t.ctx)

	loopTask := func() {
		daemonData.CommitPending(t.ctx)
		if msg, err := daemonData.GetHbMessage(t.ctx); err != nil {
			t.log.Error().Err(err).Msg("can't queue hb message")
		} else {
			var needSend bool
			newGens := hbcache.LocalGens()
			if msg.Kind != "patch" {
				needSend = true
			} else if len(newGens) != len(sentGens) {
				needSend = true
			} else {
				for n, v := range hbcache.LocalGens() {
					if sentGens[n] != v {
						needSend = true
						break
					}
				}
			}

			if needSend {
				t.log.Debug().Msgf("queue a new hb message %s gen %v", msg.Kind, newGens)
				daemonctx.HBSendQ(t.ctx) <- msg
				sentGens = newGens
			} else {
				t.log.Debug().Msgf("already queued message %s %v", msg.Kind, sentGens)
			}
		}
	}

	loopTask()
	for {
		select {
		case <-t.ctx.Done():
			t.log.Info().Msg("loop stopped")
			return
		case <-ticker.C:
			loopTask()
		}
	}
}
