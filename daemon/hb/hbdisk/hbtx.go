package hbdisk

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/daemon/daemonlogctx"
	"github.com/opensvc/om3/daemon/hb/hbctrl"
	"github.com/opensvc/om3/util/hostname"
)

type (
	tx struct {
		sync.WaitGroup
		base     base
		ctx      context.Context
		id       string
		nodes    []string
		timeout  time.Duration
		interval time.Duration

		name   string
		log    zerolog.Logger
		cmdC   chan<- interface{}
		msgC   chan<- *hbtype.Msg
		cancel func()
	}
)

// Id implements the Id function of Transmitter interface for tx
func (t *tx) Id() string {
	return t.id
}

// Stop implements the Stop function of Transmitter interface for tx
func (t *tx) Stop() error {
	t.log.Debug().Msg("cancelling")
	t.cancel()
	for _, node := range t.nodes {
		t.cmdC <- hbctrl.CmdDelWatcher{
			HbId:     t.id,
			Nodename: node,
		}
	}
	t.Wait()
	t.log.Debug().Msg("wait done")
	return nil
}

// Start implements the Start function of Transmitter interface for tx
func (t *tx) Start(cmdC chan<- interface{}, msgC <-chan []byte) error {
	if err := t.base.device.open(); err != nil {
		return err
	}
	if err := t.base.LoadPeerConfig(t.nodes); err != nil {
		return err
	}
	reasonTick := fmt.Sprintf("send msg (interval %s)", t.interval)
	ctx, cancel := context.WithCancel(t.ctx)
	t.cancel = cancel
	t.cmdC = cmdC
	t.Add(1)
	go func() {
		defer t.Done()
		t.log.Info().Msg("started")
		defer t.log.Info().Msg("stopped")
		for _, node := range t.nodes {
			cmdC <- hbctrl.CmdAddWatcher{
				HbId:     t.id,
				Nodename: node,
				Ctx:      ctx,
				Timeout:  t.timeout,
			}
		}
		var b []byte
		ticker := time.NewTicker(t.interval)
		defer ticker.Stop()
		var reason string
		for {
			select {
			case <-ctx.Done():
				return
			case b = <-msgC:
				reason = "send msg"

				// No need to send the next message before a full ticker period.
				ticker.Reset(t.interval)
			case <-ticker.C:
				reason = reasonTick
			}
			if len(b) == 0 {
				continue
			}
			t.log.Debug().Msg(reason)
			t.send(b)
		}
	}()
	return nil
}

func (t *tx) send(b []byte) {
	meta, err := t.base.GetPeer(hostname.Hostname())
	if err != nil {
		t.log.Debug().Err(err).Msg("write")
		return
	}
	if t.base.WriteDataSlot(meta.Slot, b); err != nil { // TODO write timeout?
		t.log.Debug().Err(err).Msg("write")
		return
	} else {
		t.log.Debug().Msgf("wrote to slot %d %s", meta.Slot, string(b))
	}
	for _, node := range t.nodes {
		t.cmdC <- hbctrl.CmdSetPeerSuccess{
			Nodename: node,
			HbId:     t.id,
			Success:  true,
		}
	}
}

func newTx(ctx context.Context, name string, nodes []string, dev string, timeout, interval time.Duration) *tx {
	id := name + ".tx"
	log := daemonlogctx.Logger(ctx).With().Str("id", id).Logger()
	return &tx{
		ctx:      ctx,
		id:       id,
		nodes:    nodes,
		timeout:  timeout,
		interval: interval,
		log:      log,
		base: base{
			log: log,
			device: device{
				path: dev,
			},
		},
	}
}
