package hbucast

import (
	"context"
	"net"
	"time"

	"github.com/rs/zerolog"

	"opensvc.com/opensvc/core/hbtype"
	"opensvc.com/opensvc/daemon/hb/hbctrl"
	"opensvc.com/opensvc/daemon/listener/encryptconn"
	"opensvc.com/opensvc/daemon/listener/mux/muxctx"
)

type (
	tx struct {
		ctx     context.Context
		id      string
		nodes   []string
		port    string
		intf    string
		timeout time.Duration

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
	t.cancel()
	for _, node := range t.nodes {
		t.cmdC <- hbctrl.CmdDelWatcher{
			HbId:     t.id,
			Nodename: node,
		}
	}
	return nil
}

// Start implements the Start function of Transmitter interface for tx
func (t *tx) Start(cmdC chan<- interface{}, msgC <-chan []byte) error {
	started := make(chan bool)
	ctx, cancel := context.WithCancel(t.ctx)
	t.cancel = cancel
	t.cmdC = cmdC
	go func() {
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
		for {
			select {
			case <-ctx.Done():
				t.log.Info().Msg("stopped")
				return
			case b := <-msgC:
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
		return
	}
	defer func() {
		_ = conn.Close()
	}()
	if err := conn.SetDeadline(time.Now().Add(t.timeout)); err != nil {
		t.log.Info().Err(err).Msg("SetDeadline")
		return
	}
	clearConn := encryptconn.New(conn)
	if _, err := clearConn.Write(b); err != nil {
		t.log.Debug().Err(err).Msg("write")
		return
	}
	t.cmdC <- hbctrl.CmdSetPeerSuccess{
		Nodename: node,
		HbId:     t.id,
		Success:  true,
	}
}

func newTx(ctx context.Context, name string, nodes []string, port, intf string, timeout time.Duration) *tx {
	id := name + ".tx"
	log := muxctx.Logger(ctx).With().Str("id", id).Logger()
	return &tx{
		ctx:     ctx,
		id:      id,
		nodes:   nodes,
		port:    port,
		intf:    intf,
		timeout: timeout,
		log:     log,
	}
}
