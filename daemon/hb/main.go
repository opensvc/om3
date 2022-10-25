package hb

import (
	"context"
	"runtime"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/core/clusterhb"
	"opensvc.com/opensvc/core/hbtype"
	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/hb/hbctrl"
	"opensvc.com/opensvc/daemon/routinehelper"
	"opensvc.com/opensvc/daemon/subdaemon"
	"opensvc.com/opensvc/util/funcopt"
)

type (
	T struct {
		*subdaemon.T
		routinehelper.TT
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
	t.log = log.Logger.With().Str("sub", "hb").Logger()
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
	t.txs = make(map[string]hbtype.Transmitter)
	t.rxs = make(map[string]hbtype.Receiver)
	return t
}

func (t *T) MainStart(ctx context.Context) error {
	data := hbctrl.New()
	go data.Start(ctx)
	msgC := make(chan *hbtype.Msg)

	err := t.start(ctx, data, msgC)
	if err != nil {
		return err
	}
	return nil
}

// start function configure and start hb#x.rx, hb#x.tx drivers
func (t *T) start(ctx context.Context, data *hbctrl.T, msgC chan *hbtype.Msg) error {
	n, err := clusterhb.New()
	if err != nil {
		return err
	}
	registeredDataC := make([]chan []byte, 0)
	ctrlCmd := data.Cmd()
	for _, h := range n.Hbs() {
		h.Configure(ctx)
		rx := h.Rx()
		ctrlCmd <- hbctrl.CmdRegister{Id: rx.Id()}
		if err := rx.Start(ctrlCmd, msgC); err != nil {
			ctrlCmd <- hbctrl.CmdSetState{Id: rx.Id(), State: "failed"}
			t.log.Error().Err(err).Msgf("starting %s", rx.Id())
			return err
		}
		t.rxs[rx.Id()] = rx

		tx := h.Tx()
		ctrlCmd <- hbctrl.CmdRegister{Id: tx.Id()}
		localDataC := make(chan []byte)
		if err := tx.Start(ctrlCmd, localDataC); err != nil {
			t.log.Error().Err(err).Msgf("starting %s", tx.Id())
			ctrlCmd <- hbctrl.CmdSetState{Id: tx.Id(), State: "failed"}
			return err
		}
		t.txs[tx.Id()] = tx
		registeredDataC = append(registeredDataC, localDataC)
	}
	go func() {
		// multiplex data messages to hb tx drivers
		var dataC <-chan []byte
		dataC = daemonctx.HBSendQ(ctx)
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
		daemonData := daemondata.FromContext(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-demoCtx.Done():
				t.log.Debug().Msgf("received message: %.2f/s, goroutines %d", count/10, runtime.NumGoroutine())
				demoCtx, cancel = context.WithTimeout(bgCtx, 10*time.Second)
				count = 0
			case msg := <-msgC:
				t.log.Debug().Msgf("received msg type %s from %s gens: %v", msg.Kind, msg.Nodename, msg.Gen)
				switch msg.Kind {
				case "patch":
					err := daemonData.ApplyPatch(msg.Nodename, msg)
					if err != nil {
						t.log.Error().Err(err).Msgf("ApplyPatch %s from %s gens: %v", msg.Kind, msg.Nodename, msg.Gen)
					}
				case "full":
					daemonData.ApplyFull(msg.Nodename, &msg.Full)
				case "ping":
					daemonData.ApplyPing(msg.Nodename)
				}
				count++
			}
		}
	}()
	return nil
}

func (t *T) MainStop() error {
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
	return nil
}
