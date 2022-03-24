package hb

import (
	"context"
	"encoding/json"
	"runtime"
	"time"

	"github.com/rs/zerolog"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/clusterhb"
	"opensvc.com/opensvc/core/hbtype"
	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/daemondatactx"
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
func pingMsg(gen map[string]uint64) ([]byte, error) {
	msg := hbtype.Msg{
		Kind:     "ping",
		Nodename: hostname.Hostname(),
		Gen:      gen,
	}
	return json.Marshal(msg)
}

// fullMsg function is for demo
func fullMsg(nodeStatus *cluster.NodeStatus) ([]byte, error) {
	// TODO for 3
	//msg := hbtype.Msg{
	//	Kind:     "full",
	//	Nodename: hostname.Hostname(),
	//	Full:     *nodeStatus,
	//	Gen:      nodeStatus.Gen,
	//}
	//return json.Marshal(msg)
	// For b2.1
	return json.Marshal(*nodeStatus)
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
		dataBus := daemondatactx.DaemonData(t.Ctx)
		for {
			gen := dataBus.GetLocalNodeStatus().Gen
			d, err := pingMsg(gen)
			if err != nil {
				return
			}
			dataC <- d
			time.Sleep(time.Second)
		}
	}()
	go func() {
		// for demo loop on sending full messages
		dataBus := daemondatactx.DaemonData(t.Ctx)
		for {
			nodeStatus := dataBus.GetLocalNodeStatus()
			d, err := fullMsg(nodeStatus)
			if err != nil {
				t.log.Debug().Err(err).Msg("create fullMsg")
				return
			}
			dataC <- d
			time.Sleep(5 * time.Second)
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
				case "full":
					dataBus.ApplyFull(msg.Nodename, &msg.Full)
				case "patch":
					dataBus.ApplyPatch(msg.Nodename, msg)
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
