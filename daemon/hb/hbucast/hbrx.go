package hbucast

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/daemon/daemonlogctx"
	"github.com/opensvc/om3/daemon/encryptconn"
	"github.com/opensvc/om3/daemon/hb/hbctrl"
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
//
// message from unexpected source addr connection are dropped (we only take care
// about messages from other cluster node)
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
	started := make(chan bool)
	t.Add(1)
	go func() {
		defer t.Done()
		otherNodeIpM := make(map[string]struct{})
		otherNodeIpL := make([]string, 0)
		resolver := net.Resolver{}

		for _, node := range t.nodes {
			cmdC <- hbctrl.CmdAddWatcher{
				HbId:     t.id,
				Nodename: node,
				Ctx:      ctx,
				Timeout:  t.timeout,
			}
			addrs, err := resolver.LookupHost(ctx, node)
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				otherNodeIpM[addr] = struct{}{}
				otherNodeIpL = append(otherNodeIpL, addr)
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
		t.log.Info().Msgf("listen to %s for connection from %s", t.addr+":"+t.port, otherNodeIpL)
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
			connAddr := strings.Split(conn.RemoteAddr().String(), ":")[0]
			if _, ok := otherNodeIpM[connAddr]; !ok {
				t.log.Warn().Msgf("drop message from unexpected connection from %s", connAddr)
				if err := conn.Close(); err != nil {
					t.log.Warn().Err(err).Msgf("close unexpected connection from %s", connAddr)
				}
				continue
			}
			if err := conn.SetDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
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
		if err := conn.Close(); err != nil {
			t.log.Warn().Err(err).Msgf("unexpected error while closing connection from %s", conn.RemoteAddr())
		}
	}()
	data := <-msgBufferChan
	defer func() { msgBufferChan <- data }()
	i, nodename, err := conn.ReadWithNode(data)
	if err != nil {
		t.log.Warn().Err(err).Msgf("read failed from %s", conn.RemoteAddr())
		return
	}
	if i >= (msgMaxSize - 10000) {
		t.log.Warn().Msgf("read huge message from node %s:%s msg size: %d", nodename, conn.RemoteAddr(), i)
	}
	msg := hbtype.Msg{}
	if err := json.Unmarshal(data[:i], &msg); err != nil {
		t.log.Warn().Err(err).Msgf("unmarshal message failed from node %s:%s", nodename, conn.RemoteAddr())
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
