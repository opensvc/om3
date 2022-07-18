package hb

import (
	"context"
	"runtime"
	"time"

	"github.com/rs/zerolog"

	"opensvc.com/opensvc/core/clusterhb"
	"opensvc.com/opensvc/core/hbtype"
	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/daemondatactx"
	"opensvc.com/opensvc/daemon/daemonlogctx"
	"opensvc.com/opensvc/daemon/hb/hbctrl"
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
		routineTrace routineTracer
		rootDaemon   subdaemon.RootManager
		txs          map[string]hbtype.Transmitter
		rxs          map[string]hbtype.Receiver
	}

	routineTracer interface {
		Trace(string) func()
		Stats() routinehelper.Stat
	}
)

func New(opts ...funcopt.O) *T {
	t := &T{}
	t.SetTracer(routinehelper.NewTracerNoop())
	if err := funcopt.Apply(t, opts...); err != nil {
		t.log.Error().Err(err).Msg("hb funcopt.Apply")
		return nil
	}
	t.log = daemonlogctx.Logger(t.ctx)
	t.T = subdaemon.New(
		subdaemon.WithName("hb"),
		subdaemon.WithMainManager(t),
		subdaemon.WithRoutineTracer(&t.TT),
		subdaemon.WithContext(t.ctx),
	)
	t.txs = make(map[string]hbtype.Transmitter)
	t.rxs = make(map[string]hbtype.Receiver)
	return t
}

func (t *T) MainStart() error {
	t.log.Info().Msg("mgr starting")
	data := hbctrl.New(t.ctx)
	go data.Start()
	msgC := make(chan *hbtype.Msg)

	err := t.start(t.ctx, data, msgC)
	if err != nil {
		return err
	}

	t.log.Info().Msg("mgr started")
	return nil
}

// start function configure and start hb#x.rx, hb#x.tx drivers
func (t *T) start(ctx context.Context, data *hbctrl.T, msgC chan *hbtype.Msg) error {
	n, err := clusterhb.New()
	if err != nil {
		return err
	}
	registeredDataC := make([]chan []byte, 0)
	for _, h := range n.Hbs() {
		h.Configure(ctx)
		rx := h.Rx()
		if err := rx.Start(data.Cmd(), msgC); err != nil {
			t.log.Error().Err(err).Msgf("starting %s", rx.Id())
			return err
		}
		t.rxs[rx.Id()] = rx

		tx := h.Tx()
		localDataC := make(chan []byte)
		if err := tx.Start(data.Cmd(), localDataC); err != nil {
			t.log.Error().Err(err).Msgf("starting %s", tx.Id())
			return err
		}
		t.txs[tx.Id()] = tx
		registeredDataC = append(registeredDataC, localDataC)
	}
	go func() {
		// multiplex data messages to hb tx drivers
		var dataC <-chan []byte
		dataC = daemonctx.HBSendQ(t.ctx)
		if dataC == nil {
			t.log.Error().Msg("unable to retrieve HBSendQ")
			return
		}
		for {
			select {
			case <-ctx.Done():
				return
			case d := <-dataC:
				for _, senderC := range registeredDataC {
					senderC <- d
				}
			}
		}
	}()

	go func() {
		// for demo handle received messages
		count := 0.0
		bgCtx := context.Background()
		demoCtx, cancel := context.WithTimeout(bgCtx, 10*time.Second)
		defer cancel()
		dataBus := daemondatactx.DaemonData(ctx)
		for {
			select {
			case <-demoCtx.Done():
				t.log.Debug().Msgf("received message: %.2f/s, goroutines %d", count/10, runtime.NumGoroutine())
				demoCtx, cancel = context.WithTimeout(bgCtx, 10*time.Second)
				count = 0
			case msg := <-msgC:
				t.log.Debug().Msgf("received msg type %s from %s gens: %v", msg.Kind, msg.Nodename, msg.Gen)
				switch msg.Kind {
				case "patch":
					err := dataBus.ApplyPatch(msg.Nodename, msg)
					if err != nil {
						t.log.Error().Err(err).Msgf("ApplyPatch %s from %s gens: %v", msg.Kind, msg.Nodename, msg.Gen)
					}
				case "full":
					dataBus.ApplyFull(msg.Nodename, &msg.Full)
				case "ping":
					dataBus.ApplyPing(msg.Nodename)
				}
				count++
			}
		}
	}()
	return nil
}

func (t *T) MainStop() error {
	t.log.Info().Msg("mgr stopping")
	for _, tx := range t.txs {
		err := tx.Stop()
		if err != nil {
			return err
		}
	}
	for _, rx := range t.rxs {
		err := rx.Stop()
		if err != nil {
			return err
		}
	}
	t.log.Info().Msg("mgr stopped")
	return nil
}
