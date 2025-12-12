package hbmcast

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/opensvc/om3/v3/core/hbtype"
	"github.com/opensvc/om3/v3/core/omcrypto"
	"github.com/opensvc/om3/v3/daemon/hb/hbcrypto"
	"github.com/opensvc/om3/v3/daemon/hb/hbctrl"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	// rx holds an hb unicast receiver
	rx struct {
		sync.WaitGroup
		ctx      context.Context
		id       string
		nodes    []string
		udpAddr  *net.UDPAddr
		intf     *net.Interface
		timeout  time.Duration
		assembly map[string]msgMap

		name   string
		log    *plog.Logger
		cmdC   chan<- interface{}
		msgC   chan<- *hbtype.Msg
		cancel func()

		crypto atomic.Pointer[omcrypto.T]
	}
	assembly map[string]msgMap
	msgMap   map[string]dataMap
	dataMap  map[int][]byte
)

// ID implements the ID function of the Receiver interface for rx
func (t *rx) ID() string {
	return t.id
}

// Stop implements the Stop function of the Receiver interface for rx
func (t *rx) Stop() error {
	t.log.Tracef("cancelling")
	t.cancel()
	for _, node := range t.nodes {
		t.cmdC <- hbctrl.CmdDelWatcher{
			HbID:     t.id,
			Nodename: node,
		}
	}
	t.Wait()
	t.log.Tracef("wait done")
	return nil
}

func (t *rx) streamPeerDesc() string {
	if t.intf != nil {
		return fmt.Sprintf("%s@%s ← *", t.udpAddr, t.intf.Name)
	} else {
		return fmt.Sprintf("%s ← *", t.udpAddr)
	}
}

// Start implements the Start function of the Receiver interface for rx
// TODO need purge too old or dropped node assembly ?
func (t *rx) Start(cmdC chan<- interface{}, msgC chan<- *hbtype.Msg) error {
	ctx, cancel := context.WithCancel(t.ctx)
	t.cmdC = cmdC
	t.msgC = msgC
	t.cancel = cancel
	t.log.Infof("starting")
	t.assembly = make(assembly)
	started := make(chan bool)
	t.Add(1)
	go func() {
		defer t.Done()
		for _, node := range t.nodes {
			cmdC <- hbctrl.CmdAddWatcher{
				HbID:     t.id,
				Nodename: node,
				Ctx:      ctx,
				Timeout:  t.timeout,
				Desc:     t.streamPeerDesc(),
			}
		}
		listener, err := net.ListenMulticastUDP("udp", t.intf, t.udpAddr)
		if err != nil {
			t.log.Errorf("listen multicast udp %s: %s", t.udpAddr, err)
			return
		}
		listener.SetReadBuffer(MaxDatagramSize)
		t.log.Infof("listen on %s", t.udpAddr)
		defer listener.Close()

		t.Add(1)
		go func() {
			defer t.Done()
			select {
			case <-ctx.Done():
				t.log.Tracef("closing listener")
				_ = listener.Close()
				t.log.Tracef("closed listener")
				t.cancel()
				return
			}
		}()
		started <- true
		t.crypto = *hbcrypto.CryptoFromContext(ctx)
		b := make([]byte, MaxDatagramSize)
		for {
			n, src, err := listener.ReadFromUDP(b)
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					t.log.Tracef("closed connection: %s", err)
					break
				}
				t.log.Infof("read: %s", err)
				// avoid fast loop
				time.Sleep(200 * time.Millisecond)
			}
			t.recv(src, n, b)
		}
		t.log.Infof("stopped")
	}()
	<-started
	t.log.Infof("started")
	return nil
}

func (t *rx) recv(src *net.UDPAddr, n int, b []byte) {
	s := fmt.Sprint(src)
	f := fragment{}
	b = b[:n]
	//fmt.Println("xx <<<\n", hex.Dump(b))
	if err := json.Unmarshal(b, &f); err != nil {
		t.log.Warnf("unmarshal fragment from src %s: %s", s, err)
		return
	}

	if f.MsgID == "" {
		t.log.Tracef("not a udp message frame")
		return
	}
	// verify message DoS
	if msgs, ok := t.assembly[s]; !ok {
		t.assembly[s] = msgMap{}
	} else if len(msgs) > MaxMessages {
		t.log.Warnf("too many pending messages from src %s => purge", s)
		t.assembly[s] = msgMap{}
	}
	msg := t.assembly[s]

	// verify fragment DoS
	if f.Total > MaxFragments {
		// fast drop (len(fragments) will exceed MaxFragments)
		t.log.Warnf("too many  fragments from src %s msg %s => drop", s, f.MsgID)
		return
	} else if fragments, ok := msg[f.MsgID]; !ok {
		msg[f.MsgID] = dataMap{}
		t.assembly[s] = msg
	} else if len(fragments) > MaxFragments {
		t.log.Warnf("too many pending message fragments from src %s msg %s => purge", s, f.MsgID)
		// TODO delete(msg, f.MsgID) ?
		msg[f.MsgID] = dataMap{}
		t.assembly[s] = msg
		return
	}

	chunks := msg[f.MsgID]
	chunks[f.Index] = f.Chunk
	msg[f.MsgID] = chunks
	t.assembly[s] = msg

	t.log.Tracef("recv: %d/%d", len(chunks), f.Total)
	if len(chunks) < f.Total {
		// more fragments to come
		return
	}

	// assemble chunks of f.MsgID from peer s
	defer func() {
		delete(msg, f.MsgID)
		t.assembly[s] = msg
	}()
	var encMsg []byte
	if f.Total > 1 {
		var message []byte
		for i := 1; i <= f.Total; i++ {
			chunk, ok := chunks[i]
			if !ok {
				t.log.Warnf("missing fragment %d in msg %s from src %s => purge", i, f.MsgID, s)
				return
			}
			message = append(message, chunk...)
		}
		encMsg = message
	} else {
		encMsg = chunks[1]
	}
	crypto := t.crypto.Load()

	b, err := crypto.Decrypt(encMsg)
	if err != nil {
		t.log.Tracef("recv: decrypting msg from %s: %s: %s", s, hex.Dump(encMsg), err)
		return
	}
	data := hbtype.Msg{}
	if err := json.Unmarshal(b, &data); err != nil {
		t.log.Warnf("can't unmarshal msg from %s: %s", s, err)
		return
	}
	if data.Nodename == hostname.Hostname() {
		t.log.Tracef("recv: drop msg from self")
		return
	}
	t.cmdC <- hbctrl.CmdSetPeerSuccess{
		Nodename: data.Nodename,
		HbID:     t.id,
		Success:  true,
	}
	t.msgC <- &data
}

func newRx(ctx context.Context, name string, nodes []string, udpAddr *net.UDPAddr, intf *net.Interface, timeout time.Duration) *rx {
	id := name + ".rx"
	return &rx{
		ctx:     ctx,
		id:      id,
		nodes:   nodes,
		udpAddr: udpAddr,
		intf:    intf,
		timeout: timeout,
		log: plog.NewDefaultLogger().Attr("pkg", "daemon/hb/hbmcast").
			Attr("hb_func", "rx").
			Attr("hb_name", name).
			Attr("hb_id", id).
			WithPrefix("daemon: hb: mcast: rx: " + name + ": "),
	}
}
