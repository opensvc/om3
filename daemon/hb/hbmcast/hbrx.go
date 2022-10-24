package hbmcast

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/rs/zerolog"

	reqjsonrpc "opensvc.com/opensvc/core/client/requester/jsonrpc"
	"opensvc.com/opensvc/core/hbtype"
	"opensvc.com/opensvc/daemon/daemonlogctx"
	"opensvc.com/opensvc/daemon/hb/hbctrl"
	"opensvc.com/opensvc/util/hostname"
)

type (
	// rx holds an hb unicast receiver
	rx struct {
		ctx      context.Context
		id       string
		nodes    []string
		udpAddr  *net.UDPAddr
		intf     string
		timeout  time.Duration
		assembly map[string]msgMap

		name   string
		log    zerolog.Logger
		cmdC   chan<- interface{}
		msgC   chan<- *hbtype.Msg
		cancel func()
	}
	assembly map[string]msgMap
	msgMap   map[string]dataMap
	dataMap  map[int][]byte
)

// Id implements the Id function of the Receiver interface for rx
func (t *rx) Id() string {
	return t.id
}

// Stop implements the Stop function of the Receiver interface for rx
func (t *rx) Stop() error {
	t.cancel()
	for _, node := range t.nodes {
		t.cmdC <- hbctrl.CmdDelWatcher{
			HbId:     t.id,
			Nodename: node,
		}
	}
	return nil
}

// Start implements the Start function of the Receiver interface for rx
func (t *rx) Start(cmdC chan<- interface{}, msgC chan<- *hbtype.Msg) error {
	ctx, cancel := context.WithCancel(t.ctx)
	t.cmdC = cmdC
	t.msgC = msgC
	t.cancel = cancel
	t.log.Info().Msg("starting")
	t.assembly = make(assembly)
	started := make(chan bool)
	go func() {
		for _, node := range t.nodes {
			cmdC <- hbctrl.CmdAddWatcher{
				HbId:     t.id,
				Nodename: node,
				Ctx:      ctx,
				Timeout:  t.timeout,
			}
		}
		listener, err := net.ListenMulticastUDP("udp", nil, t.udpAddr)
		if err != nil {
			t.log.Error().Err(err).Msgf("listen multicast udp %s", t.udpAddr)
			return
		}
		listener.SetReadBuffer(MaxDatagramSize)
		t.log.Info().Msgf("listen on %s", t.udpAddr)
		defer listener.Close()

		go func() {
			select {
			case <-ctx.Done():
				t.log.Info().Msg("closing listener")
				listener.Close()
				t.log.Info().Msg("closed listener")
				t.cancel()
				return
			}
		}()
		started <- true
		for {
			b := make([]byte, MaxDatagramSize)
			n, src, err := listener.ReadFromUDP(b)
			if err != nil {
				t.log.Info().Err(err).Msg("ReadFromUDP")
			}
			t.recv(src, n, b)
		}
		t.log.Info().Msg("stopped")
	}()
	<-started
	t.log.Info().Msg("started")
	return nil
}

func (t *rx) recv(src *net.UDPAddr, n int, b []byte) {
	s := fmt.Sprint(src)
	f := fragment{}
	b = b[:n]
	//fmt.Println("xx <<<\n", hex.Dump(b))
	if err := json.Unmarshal(b, &f); err != nil {
		t.log.Warn().Err(err).Msgf("umarshal fragment from src %s", s)
		return
	}

	if f.MsgID == "" {
		t.log.Debug().Msg("not a udp message frame")
		return
	}
	// verify message DoS
	if msgs, ok := t.assembly[s]; !ok {
		t.assembly[s] = msgMap{}
	} else if len(msgs) > MaxMessages {
		t.log.Warn().Msgf("too many pending messages from src %s. purge", s)
		t.assembly[s] = msgMap{}
	}
	msg := t.assembly[s]

	// verify fragment DoS
	if fragments, ok := msg[f.MsgID]; !ok {
		msg[f.MsgID] = dataMap{}
		t.assembly[s] = msg
	} else if len(fragments) > MaxFragments {
		t.log.Warn().Msgf("too many pending message fragments from src %s msg %s. purge", s, f.MsgID)
		msg[f.MsgID] = dataMap{}
		t.assembly[s] = msg
		return
	}

	chunks := msg[f.MsgID]
	chunks[f.Index] = f.Chunk
	msg[f.MsgID] = chunks
	t.assembly[s] = msg

	t.log.Debug().Msgf("recv: %d/%d", len(chunks), f.Total)
	if len(chunks) < f.Total {
		// more fragments to come
		return
	}

	// assemble chunks
	message := []byte{}
	for i := 1; i <= f.Total; i += 1 {
		chunk, ok := chunks[i]
		if !ok {
			t.log.Warn().Msgf("missing fragment %d in msg %s from src. purge", i, f.MsgID, s)
			delete(msg, f.MsgID)
			t.assembly[s] = msg
			return
		}
		message = append(message, chunk...)
	}

	encMsg := reqjsonrpc.NewMessage(message)
	b, _, err := encMsg.DecryptWithNode()
	if err != nil {
		t.log.Debug().Err(err).Msgf("recv: decrypting msg from %s: %s", s, hex.Dump(message))
		return
	}

	data := hbtype.Msg{}
	if err := json.Unmarshal(b, &data); err != nil {
		t.log.Warn().Err(err).Msgf("can't unmarshal msg from %s", s)
		return
	}
	if data.Nodename == hostname.Hostname() {
		t.log.Debug().Msg("recv: drop msg from self")
		return
	}
	t.cmdC <- hbctrl.CmdSetPeerSuccess{
		Nodename: data.Nodename,
		HbId:     t.id,
		Success:  true,
	}
	t.msgC <- &data
	delete(msg, f.MsgID)
	t.assembly[s] = msg
}

func newRx(ctx context.Context, name string, nodes []string, udpAddr *net.UDPAddr, intf string, timeout time.Duration) *rx {
	id := name + ".rx"
	log := daemonlogctx.Logger(ctx).With().Str("id", id).Logger()
	return &rx{
		ctx:     ctx,
		id:      id,
		nodes:   nodes,
		udpAddr: udpAddr,
		intf:    intf,
		timeout: timeout,
		log:     log,
	}
}
