package hbucast

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"opensvc.com/opensvc/core/hbtype"
	"opensvc.com/opensvc/daemon/daemonlogctx"
	"opensvc.com/opensvc/daemon/hb/hbctrl"
	"opensvc.com/opensvc/daemon/listener/encryptconn"
)

type (
	// rx holds an hb unicast receiver
	rx struct {
		sync.WaitGroup
		ctx     context.Context
		id      string
		nodes   []string
		addr    string
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

var (
	msgBufferCount = 4
	msgMaxSize     = 10000000 // max kind=full msg size
	msgBufferChan  = make(chan []byte, msgBufferCount)
)

func init() {
	// Use cached buffers to reduce cpu when many message are scanned
	for i := 0; i < msgBufferCount; i++ {
		b := make([]byte, msgMaxSize)
		msgBufferChan <- b
	}
}

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
func (t *rx) Start(cmdC chan<- interface{}, msgC chan<- *hbtype.Msg) error {
	ctx, cancel := context.WithCancel(t.ctx)
	t.cmdC = cmdC
	t.msgC = msgC
	t.cancel = cancel
	t.log.Info().Msg("starting")
	listener, err := net.Listen("tcp", t.addr+":"+t.port)
	if err != nil {
		t.log.Error().Err(err).Msg("listen failed")
		return err
	}
	t.log.Info().Msgf("listen on %s", t.addr+":"+t.port)
	started := make(chan bool)
	t.Add(1)
	go func() {
		defer t.Done()
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
			select {
			case <-ctx.Done():
				t.log.Debug().Msg("closing listener")
				_ = listener.Close()
				t.log.Debug().Msg("closed listener")
				t.cancel()
				return
			}
		}()
		started <- true
		for {
			conn, err := listener.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					break
				} else {
					t.log.Error().Err(err).Msg("Accept")
					continue
				}
			}
			if err := conn.SetDeadline(time.Now().Add(t.timeout)); err != nil {
				t.log.Info().Err(err).Msg("SetReadDeadline")
				continue
			}
			clearConn := encryptconn.New(conn)
			t.Add(1)
			go t.handle(clearConn)
		}
		t.log.Info().Msg("stopped " + t.addr)
	}()
	<-started
	t.log.Info().Msg("started " + t.addr)
	return nil
}

func (t *rx) handle(conn encryptconn.ConnNoder) {
	defer t.Done()
	defer func() {
		_ = conn.Close()
	}()
	data := <-msgBufferChan
	defer func() { msgBufferChan <- data }()
	i, nodename, err := conn.ReadWithNode(data)
	if err != nil {
		t.log.Error().Err(err).Msg("ReadWithNode failure")
		return
	}
	if i >= (msgMaxSize - 10000) {
		t.log.Warn().Msgf("ReadWithNode huge message from %s: %d", nodename, i)
	}
	msg := hbtype.Msg{}
	if err := json.Unmarshal(data[:i], &msg); err != nil {
		t.log.Warn().Err(err).Msgf("can't unmarshal msg from %s", nodename)
		return
	}
	t.cmdC <- hbctrl.CmdSetPeerSuccess{
		Nodename: msg.Nodename,
		HbId:     t.id,
		Success:  true,
	}
	t.msgC <- &msg
}

func newRx(ctx context.Context, name string, nodes []string, addr, port, intf string, timeout time.Duration) *rx {
	id := name + ".rx"
	log := daemonlogctx.Logger(ctx).With().Str("id", id).Logger()
	return &rx{
		ctx:     ctx,
		id:      id,
		nodes:   nodes,
		addr:    addr,
		port:    port,
		intf:    intf,
		timeout: timeout,
		log:     log,
	}
}
