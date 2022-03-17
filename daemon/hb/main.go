package hb

import (
	"context"
	"encoding/json"
	"runtime"
	"time"

	"github.com/rs/zerolog"

	"opensvc.com/opensvc/core/clusterhb"
	"opensvc.com/opensvc/core/hbtype"
	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/hb/hbctrl"
	"opensvc.com/opensvc/util/hostname"

	"opensvc.com/opensvc/daemon/routinehelper"
	"opensvc.com/opensvc/daemon/subdaemon"
	"opensvc.com/opensvc/util/funcopt"
)

type (
	T struct {
		*subdaemon.T
		daemonctx.TCtx
		log          zerolog.Logger
		routineTrace routineTracer
		rootDaemon   subdaemon.RootManager
		routinehelper.TT
		txs map[string]hbtype.Transmitter
		rxs map[string]hbtype.Receiver
	}

	routineTracer interface {
		Trace(string) func()
		Stats() routinehelper.Stat
	}
)

func New(opts ...funcopt.O) *T {
	t := &T{TCtx: daemonctx.TCtx{}}
	t.SetTracer(routinehelper.NewTracerNoop())
	if err := funcopt.Apply(t, opts...); err != nil {
		t.log.Error().Err(err).Msg("hb funcopt.Apply")
		return nil
	}
	t.T = subdaemon.New(
		subdaemon.WithName("hb"),
		subdaemon.WithMainManager(t),
		subdaemon.WithRoutineTracer(&t.TT),
	)
	t.log = t.Log()
	t.txs = make(map[string]hbtype.Transmitter)
	t.rxs = make(map[string]hbtype.Receiver)
	return t
}

// pingMsg function is for demo
func pingMsg() ([]byte, error) {
	msg := hbtype.Msg{
		Kind:     "ping",
		Nodename: hostname.Hostname(),
	}
	return json.Marshal(msg)
}

func (t *T) MainStart() error {
	t.log.Info().Msg("mgr starting")
	data := hbctrl.New(t.Ctx)
	go data.Start()
	msgC := make(chan *hbtype.Msg)

	err := t.start(t.Ctx, data, msgC)
	if err != nil {
		return err
	}

	t.log.Info().Msg("mgr started")
	return nil
}

// start function configure and start hb#x.rx, hb#x.tx drivers
func (t *T) start(ctx context.Context, data *hbctrl.T, msgC chan *hbtype.Msg) error {
	n := clusterhb.New()
	registeredDataC := make([]chan []byte, 0)
	dataC := make(chan []byte)
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
		// for demo loop on sending ping messages
		for {
			d, _ := pingMsg()
			dataC <- d
			time.Sleep(time.Second)
		}
	}()
	go func() {
		// for demo handle received messages
		count := 0.0
		bgCtx := context.Background()
		ctx, _ := context.WithTimeout(bgCtx, 10*time.Second)
		for {
			select {
			case <-ctx.Done():
				t.log.Info().Msgf("received message: %.2f/s, goroutines %d", count/10, runtime.NumGoroutine())
				ctx, _ = context.WithTimeout(bgCtx, 10*time.Second)
				count = 0
			case <-msgC:
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
