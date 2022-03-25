package hb

import (
	"context"
	"encoding/json"
	"runtime"
	"strconv"
	"time"

	"github.com/rs/zerolog"

	"opensvc.com/opensvc/core/clusterhb"
	"opensvc.com/opensvc/core/hbtype"
	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/daemondatactx"
	"opensvc.com/opensvc/daemon/hb/hbctrl"
	"opensvc.com/opensvc/daemon/routinehelper"
	"opensvc.com/opensvc/daemon/subdaemon"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/jsondelta"
	"opensvc.com/opensvc/util/timestamp"
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

	// For demo
	go t.sendPing(dataC, 1, 5*time.Second)
	time.Sleep(2 * time.Second)
	go t.sendFull(dataC, 1, 5*time.Second)
	time.Sleep(2 * time.Second)
	go t.sendPatch(dataC, 100000000, 1*time.Microsecond)

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
				case "full":
					dataBus.ApplyFull(msg.Nodename, &msg.Full)
				case "patch":
					err := dataBus.ApplyPatch(msg.Nodename, msg)
					if err != nil {
						t.log.Error().Err(err).Msgf("ApplyPatch %s from %s gens: %v", msg.Kind, msg.Nodename, msg.Gen)
					}
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

func (t *T) sendPing(dataC chan<- []byte, count int, interval time.Duration) {
	// for demo loop on sending ping messages
	dataBus := daemondatactx.DaemonData(t.Ctx)
	for i := 0; i < count; i++ {
		nodeStatus := dataBus.GetLocalNodeStatus()
		msg := hbtype.Msg{
			Kind:     "ping",
			Nodename: hostname.Hostname(),
			Gen:      nodeStatus.Gen,
		}
		d, err := json.Marshal(msg)
		if err != nil {
			return
		}
		t.log.Error().Msgf("send ping message %v", nodeStatus.Gen)
		dataC <- d
		time.Sleep(interval)
	}
}

func (t *T) sendFull(dataC chan<- []byte, count int, interval time.Duration) {
	// for demo loop on sending full messages
	dataBus := daemondatactx.DaemonData(t.Ctx)
	for i := 0; i < count; i++ {
		nodeStatus := dataBus.GetLocalNodeStatus()
		// TODO for 3
		//msg := hbtype.Msg{
		//	Kind:     "full",
		//	Nodename: hostname.Hostname(),
		//	Full:     *nodeStatus,
		//	Gen:      nodeStatus.Gen,
		//}
		//return json.Marshal(msg)
		// For b2.1
		d, err := json.Marshal(*nodeStatus)
		if err != nil {
			t.log.Debug().Err(err).Msg("create fullMsg")
			return
		}
		t.log.Error().Msgf("send fullMsg %v", nodeStatus.Gen)
		dataC <- d
		time.Sleep(interval)
	}
}

func (t *T) sendPatch(dataC chan<- []byte, count int, interval time.Duration) {
	// for demo loop on sending patch messages
	dataBus := daemondatactx.DaemonData(t.Ctx)
	localhost := hostname.Hostname()
	for i := 0; i < count; i++ {
		ops := make([]jsondelta.Operation, 0)
		dataBus.CommitPending()
		localNodeStatus := dataBus.GetLocalNodeStatus()
		newGen := localNodeStatus.Gen[localhost]
		newGen++
		localNodeStatus.Gen[localhost] = newGen
		ops = append(ops, jsondelta.Operation{
			OpPath:  []interface{}{"gen", localhost},
			OpValue: jsondelta.NewOptValue(newGen),
			OpKind:  "replace",
		})
		ops = append(ops, jsondelta.Operation{
			OpPath:  []interface{}{"updated"},
			OpValue: jsondelta.NewOptValue(timestamp.Now()),
			OpKind:  "replace",
		})
		patch := hbtype.Msg{
			Kind: "patch",
			Gen:  localNodeStatus.Gen,
			Deltas: map[string]jsondelta.Patch{
				strconv.FormatUint(newGen, 10): ops,
			},
			Nodename: localhost,
		}
		err := dataBus.ApplyPatch(localhost, &patch)
		if err != nil {
			t.log.Error().Err(err).Msgf("ApplyPatch node gen %d", newGen)
		}
		dataBus.CommitPending()
		if b, err := json.Marshal(patch); err == nil {
			t.log.Debug().Msgf("Send new patch %d: %s", newGen, b)
			dataC <- b
		}
		time.Sleep(interval)
	}
}
