package hbdisk

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/rs/zerolog"

	reqjsonrpc "opensvc.com/opensvc/core/client/requester/jsonrpc"
	"opensvc.com/opensvc/core/hbtype"
	"opensvc.com/opensvc/daemon/daemonlogctx"
	"opensvc.com/opensvc/daemon/hb/hbctrl"
)

type (
	// rx holds an hb unicast receiver
	rx struct {
		sync.WaitGroup
		base     base
		ctx      context.Context
		id       string
		nodes    []string
		timeout  time.Duration
		interval time.Duration
		last     time.Time

		name   string
		log    zerolog.Logger
		cmdC   chan<- any
		msgC   chan<- *hbtype.Msg
		cancel func()
	}
)

// Id implements the Id function of the Receiver interface for rx
func (t *rx) Id() string {
	return t.id
}

// Stop implements the Stop function of the Receiver interface for rx
func (t *rx) Stop() error {
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

// Start implements the Start function of the Receiver interface for rx
func (t *rx) Start(cmdC chan<- any, msgC chan<- *hbtype.Msg) error {
	ctx, cancel := context.WithCancel(t.ctx)
	t.cmdC = cmdC
	t.msgC = msgC
	t.cancel = cancel
	ticker := time.NewTicker(t.interval)

	for _, node := range t.nodes {
		cmdC <- hbctrl.CmdAddWatcher{
			HbId:     t.id,
			Nodename: node,
			Ctx:      ctx,
			Timeout:  t.timeout,
		}
	}

	t.Add(1)
	go func() {
		defer t.Done()
		defer ticker.Stop()
		t.log.Info().Msg("started")
		for {
			select {
			case <-ticker.C:
				t.onTick()
			case <-ctx.Done():
				t.cancel()
				break
			}
		}
		t.log.Info().Msg("stopped")
	}()
	return nil
}

func (t *rx) onTick() {
	for _, node := range t.nodes {
		t.recv(node)
	}
}

func (t *rx) recv(nodename string) {
	meta, err := t.base.GetPeer(nodename)
	if err != nil {
		t.log.Debug().Err(err).Msgf("recv: failed to allocate a slot for node %s", nodename)
		return
	}
	c, err := t.base.ReadDataSlot(meta.Slot) // TODO read timeout?
	if err != nil {
		t.log.Debug().Err(err).Msgf("recv: reading node %s data slot %d", nodename, meta.Slot)
		return
	}
	if c.Updated.IsZero() {
		t.log.Debug().Msgf("recv: node %s data slot %d has never been updated", nodename, meta.Slot)
		return
	}
	if !t.last.IsZero() && c.Updated == t.last {
		t.log.Debug().Msgf("recv: node %s data slot %d has not change since last read", nodename, meta.Slot)
		return
	}
	elapsed := time.Now().Sub(c.Updated)
	if elapsed > t.timeout {
		t.log.Debug().Msgf("recv: node %s data slot %d has not been updated for %s", nodename, meta.Slot, elapsed)
		return
	}
	encMsg := reqjsonrpc.NewMessage(c.Msg)
	b, msgNodename, err := encMsg.DecryptWithNode()
	if err != nil {
		t.log.Debug().Err(err).Msgf("recv: decrypting node %s data slot %d", nodename, meta.Slot)
		return
	}

	if nodename != msgNodename {
		t.log.Debug().Err(err).Msgf("recv: node %s data slot %d was written by unexpected node %s", nodename, meta.Slot, msgNodename)
		return
	}

	msg := hbtype.Msg{}
	if err := json.Unmarshal(b, &msg); err != nil {
		t.log.Warn().Err(err).Msgf("can't unmarshal msg from %s", nodename)
		return
	}
	t.log.Debug().Msgf("recv: node %s unmarshaled %#v", nodename, msg)
	t.cmdC <- hbctrl.CmdSetPeerSuccess{
		Nodename: msg.Nodename,
		HbId:     t.id,
		Success:  true,
	}
	t.msgC <- &msg
	t.last = c.Updated
}

func newRx(ctx context.Context, name string, nodes []string, baba base, timeout, interval time.Duration) *rx {
	id := name + ".rx"
	log := daemonlogctx.Logger(ctx).With().Str("id", id).Logger()
	return &rx{
		ctx:      ctx,
		id:       id,
		nodes:    nodes,
		timeout:  timeout,
		interval: interval,
		log:      log,
		base:     baba,
	}
}
