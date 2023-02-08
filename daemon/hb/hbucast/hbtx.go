package hbucast

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/daemon/daemonlogctx"
	"github.com/opensvc/om3/daemon/hb/hbctrl"
)

type (
	tx struct {
		sync.WaitGroup
		ctx      context.Context
		id       string
		nodes    []string
		port     string
		intf     string
		interval time.Duration
		timeout  time.Duration

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
	started := make(chan bool)
	ctx, cancel := context.WithCancel(t.ctx)
	t.cancel = cancel
	t.cmdC = cmdC
	t.Add(1)
	go func() {
		defer t.Done()
		t.log.Info().Msg("starting")
		for _, node := range t.nodes {
			cmdC <- hbctrl.CmdAddWatcher{
				HbId:     t.id,
				Nodename: node,
				Ctx:      ctx,
				Timeout:  t.timeout,
			}
		}
		started <- true
		var b []byte
		ticker := time.NewTicker(t.interval)
		defer ticker.Stop()
		var reason string
		for {
			select {
			case <-ctx.Done():
				t.log.Info().Msg("stopped")
				return
			case b = <-msgC:
				reason = "send msg"
				ticker.Reset(t.interval)
			case <-ticker.C:
				reason = "send msg (interval)"
			}
			if len(b) == 0 {
				continue
			} else {
				t.log.Debug().Msg(reason)
				for _, node := range t.nodes {
					go t.send(node, b)
				}
			}
		}
	}()
	<-started
	t.log.Info().Msg("started")
	return nil
}

func (t *tx) send(node string, b []byte) {
	conn, err := net.DialTimeout("tcp", node+":"+t.port, t.timeout)
	if err != nil {
		t.log.Debug().Err(err).Msg("DialTimeout")
		return
	}
	defer func() {
		_ = conn.Close()
	}()
	if err := conn.SetDeadline(time.Now().Add(t.timeout)); err != nil {
		t.log.Error().Err(err).Msg("SetDeadline")
		return
	}
	if n, err := conn.Write(b); err != nil {
		t.log.Debug().Err(err).Msg("write")
		return
	} else if n != len(b) {
		t.log.Debug().Msgf("write %d instead of %d", n, len(b))
		return
	}
	t.cmdC <- hbctrl.CmdSetPeerSuccess{
		Nodename: node,
		HbId:     t.id,
		Success:  true,
	}
}

func newTx(ctx context.Context, name string, nodes []string, port, intf string, timeout, interval time.Duration) *tx {
	id := name + ".tx"
	log := daemonlogctx.Logger(ctx).With().Str("id", id).Logger()
	return &tx{
		ctx:      ctx,
		id:       id,
		nodes:    nodes,
		port:     port,
		intf:     intf,
		interval: interval,
		timeout:  timeout,
		log:      log,
	}
}
