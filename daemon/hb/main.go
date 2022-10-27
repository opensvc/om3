package hb

import (
	"context"
	"runtime"
	"strings"
	"time"

	"github.com/pkg/errors"
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

		ctrlC        chan<- any
		readMsgQueue chan *hbtype.Msg

		registerTxC   chan registerTxQueue
		unregisterTxC chan string
	}

	registerTxQueue struct {
		id string
		// msgToSendQueue is the queue on which a tx fetch messages to send
		msgToSendQueue chan []byte
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
	t.readMsgQueue = make(chan *hbtype.Msg)
	return t
}

// MainStart starts heartbeat components
//
// It starts:
// - the hb controller to maintain heartbeat status and peers
// - the dispatcher of messages to send to hb tx components
// - the dispatcher of read messages from hb rx components to daemon data
// - the launcher of tx, rx components found in configuration
func (t *T) MainStart(ctx context.Context) error {
	t.ctrlC = hbctrl.Start(ctx)

	err := t.msgToTx(ctx)
	if err != nil {
		return err
	}

	go t.msgFromRx(ctx)

	err = t.janitorHb(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (t *T) MainStop() error {
	hbToStop := make([]hbtype.IdStopper, 0)
	var failedIds []string
	for _, hb := range t.txs {
		hbToStop = append(hbToStop, hb)
	}
	for _, hb := range t.rxs {
		hbToStop = append(hbToStop, hb)
	}
	for _, hb := range hbToStop {
		if err := t.stopHb(hb); err != nil {
			t.log.Error().Err(err).Msgf("failure during stop %s", hb.Id())
			failedIds = append(failedIds, hb.Id())
		}
	}
	if len(failedIds) > 0 {
		return errors.New("failure while stopping heartbeat " + strings.Join(failedIds, ", "))
	}
	return nil
}

func (t *T) stopHb(hb hbtype.IdStopper) error {
	switch hb.(type) {
	case hbtype.Transmitter:
		t.unregisterTxC <- hb.Id()
	}
	return hb.Stop()
}

// msgToTx starts a msg multiplexer data messages to hb tx drivers
func (t *T) msgToTx(ctx context.Context) error {
	dataC := daemonctx.HBSendQ(ctx)
	if dataC == nil {
		return errors.New("msgToTx unable to retrieve HBSendQ")
	}
	t.registerTxC = make(chan registerTxQueue)
	t.unregisterTxC = make(chan string)
	go func() {
		registeredTxMsgQueue := make(map[string]chan []byte)
		for {
			select {
			case <-ctx.Done():
				return
			case c := <-t.registerTxC:
				t.log.Debug().Msgf("add %s to hb transmitters", c.id)
				registeredTxMsgQueue[c.id] = c.msgToSendQueue
			case txId := <-t.unregisterTxC:
				t.log.Debug().Msgf("remove %s from hb transmitters", txId)
				delete(registeredTxMsgQueue, txId)
			case d := <-dataC:
				for _, txQueue := range registeredTxMsgQueue {
					txQueue <- d
				}
			}
		}
	}()
	return nil
}

func (t *T) msgFromRx(ctx context.Context) {
	count := 0.0
	statTicker := time.NewTicker(10 * time.Second)
	defer statTicker.Stop()
	daemonData := daemondata.FromContext(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-statTicker.C:
			t.log.Debug().Msgf("received message: %.2f/s, goroutines %d", count/10, runtime.NumGoroutine())
			count = 0
		case msg := <-t.readMsgQueue:
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
}

func (t *T) janitorHb(ctx context.Context) error {
	n, err := clusterhb.New()
	if err != nil {
		return err
	}
	go func() {
		for _, h := range n.Hbs() {
			h.Configure(ctx)
			rx := h.Rx()
			if rx == nil {
				continue
			}
			t.ctrlC <- hbctrl.CmdRegister{Id: rx.Id()}
			if err := rx.Start(t.ctrlC, t.readMsgQueue); err != nil {
				t.ctrlC <- hbctrl.CmdSetState{Id: rx.Id(), State: "failed"}
				t.log.Error().Err(err).Msgf("starting %s", rx.Id())
				continue
			}
			t.rxs[rx.Id()] = rx

			tx := h.Tx()
			if rx == nil {
				continue
			}
			t.ctrlC <- hbctrl.CmdRegister{Id: tx.Id()}
			localDataC := make(chan []byte)
			if err := tx.Start(t.ctrlC, localDataC); err != nil {
				t.log.Error().Err(err).Msgf("starting %s", tx.Id())
				t.ctrlC <- hbctrl.CmdSetState{Id: tx.Id(), State: "failed"}
				continue
			}
			t.registerTxC <- registerTxQueue{id: tx.Id(), msgToSendQueue: localDataC}
			t.txs[tx.Id()] = tx
		}
	}()
	return nil
}
