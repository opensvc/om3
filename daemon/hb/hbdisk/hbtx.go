package hbdisk

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"

	reqjsonrpc "opensvc.com/opensvc/core/client/requester/jsonrpc"
	"opensvc.com/opensvc/core/hbtype"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/daemon/daemonlogctx"
	"opensvc.com/opensvc/daemon/hb/hbctrl"
	"opensvc.com/opensvc/util/hostname"
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
	ctx, cancel := context.WithCancel(t.ctx)
	t.cancel = cancel
	t.cmdC = cmdC
	t.Add(1)
	go func() {
		defer t.Done()
		t.log.Info().Msg("started")
		for _, node := range t.nodes {
			cmdC <- hbctrl.CmdAddWatcher{
				HbId:     t.id,
				Nodename: node,
				Ctx:      ctx,
				Timeout:  t.timeout,
			}
		}
		for {
			select {
			case <-ctx.Done():
				break
			case b := <-msgC:
				t.send(b)
			}
		}
		t.log.Info().Msg("stopped")
	}()
	return nil
}

func (t *tx) slotData(b []byte) ([]byte, error) {
	cluster := rawconfig.ClusterSection()
	msg := &reqjsonrpc.Message{
		NodeName:    hostname.Hostname(),
		ClusterName: cluster.Name,
		Key:         cluster.Secret,
		Data:        b,
	}
	return msg.Encrypt()
}

func (t *tx) send(b []byte) {
	slotData, err := t.slotData(b)
	if err != nil {
		t.log.Debug().Err(err).Msg("write")
		return
	}
	meta, err := t.base.GetPeer(hostname.Hostname())
	if err != nil {
		t.log.Debug().Err(err).Msg("write")
		return
	}
	if t.base.WriteDataSlot(meta.Slot, slotData); err != nil { // TODO write timeout?
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

func newTx(ctx context.Context, name string, nodes []string, baba base, timeout, interval time.Duration) *tx {
	id := name + ".tx"
	log := daemonlogctx.Logger(ctx).With().Str("id", id).Logger()
	return &tx{
		ctx:      ctx,
		id:       id,
		nodes:    nodes,
		timeout:  timeout,
		interval: interval,
		log:      log,
		base:     baba,
	}
}
