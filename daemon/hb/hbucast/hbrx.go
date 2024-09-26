package hbucast

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/opensvc/om3/core/clusterdump"
	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/core/omcrypto"
	"github.com/opensvc/om3/daemon/encryptconn"
	"github.com/opensvc/om3/daemon/hb/hbctrl"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/plog"
)

type (
	// rx holds a hb unicast receiver
	rx struct {
		sync.WaitGroup
		ctx     context.Context
		id      string
		nodes   map[string]string
		addr    string
		port    string
		intf    string
		timeout time.Duration

		name   string
		log    *plog.Logger
		cmdC   chan<- interface{}
		msgC   chan<- *hbtype.Msg
		cancel func()
	}
)

var (
	// messageTimeout
	messageTimeout = 500 * time.Millisecond

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

// ID implements the ID function of the Receiver interface for rx
func (t *rx) ID() string {
	return t.id
}

// Stop implements the Stop function of the Receiver interface for rx
func (t *rx) Stop() error {
	t.log.Debugf("cancelling")
	t.cancel()
	for node := range t.nodes {
		t.cmdC <- hbctrl.CmdDelWatcher{
			HbID:     t.id,
			Nodename: node,
		}
	}
	t.Wait()
	t.log.Debugf("wait done")
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
	t.log.Infof("starting: timeout %s", t.timeout)
	listener, err := net.Listen("tcp", t.addr+":"+t.port)
	if err != nil {
		t.log.Errorf("listen failed: %s", err)
		return err
	}
	started := make(chan bool)
	t.Add(1)
	go func() {
		defer t.Done()
		otherNodeIPM := make(map[string]struct{})
		otherNodeIPL := make([]string, 0)
		resolver := net.Resolver{}

		for node, addr := range t.nodes {
			cmdC <- hbctrl.CmdAddWatcher{
				HbID:     t.id,
				Nodename: node,
				Ctx:      ctx,
				Timeout:  t.timeout,
			}
			addrs, err := resolver.LookupHost(ctx, addr)
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				otherNodeIPM[addr] = struct{}{}
				otherNodeIPL = append(otherNodeIPL, addr)
			}
		}
		t.Add(1)
		go func() {
			defer t.Done()
			select {
			case <-ctx.Done():
				t.log.Debugf("closing listener")
				_ = listener.Close()
				t.log.Debugf("closed listener")
				t.cancel()
				return
			}
		}()
		t.log.Infof("listen to %s for connection from %s", t.addr+":"+t.port, otherNodeIPL)
		started <- true
		for {
			conn, err := listener.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					break
				} else {
					t.log.Errorf("listener accept: %s", err)
					continue
				}
			}
			connAddr := strings.Split(conn.RemoteAddr().String(), ":")[0]
			if _, ok := otherNodeIPM[connAddr]; !ok {
				t.log.Warnf("drop message from unexpected connection from %s", connAddr)
				if err := conn.Close(); err != nil {
					t.log.Warnf("close unexpected connection from %s: %s", connAddr, err)
				}
				continue
			}
			if err := conn.SetDeadline(time.Now().Add(messageTimeout)); err != nil {
				t.log.Infof("can't set read deadline for %s: %s", connAddr, err)
				continue
			}
			clusterConfig := clusterdump.ConfigData.Get()
			clearConn := encryptconn.New(conn, &omcrypto.Factory{
				NodeName:    hostname.Hostname(),
				ClusterName: clusterConfig.Name,
				Key:         clusterConfig.Secret(),
			})
			t.Add(1)
			go t.handle(clearConn)
		}
		t.log.Infof("stopped %s", t.addr)
	}()
	<-started
	t.log.Infof("started %s", t.addr)
	return nil
}

func (t *rx) handle(conn encryptconn.ConnNoder) {
	defer t.Done()
	defer func() {
		if err := conn.Close(); err != nil {
			t.log.Warnf("unexpected error while closing connection from %s: %s", conn.RemoteAddr(), err)
		}
	}()
	data := <-msgBufferChan
	defer func() { msgBufferChan <- data }()
	i, nodename, err := conn.ReadWithNode(data)
	if err != nil {
		t.log.Warnf("read failed from %s: %s", conn.RemoteAddr(), err)
		return
	}
	if i >= (msgMaxSize - 10000) {
		t.log.Warnf("read huge message from node %s:%s msg size: %d", nodename, conn.RemoteAddr(), i)
	}
	msg := hbtype.Msg{}
	if err := json.Unmarshal(data[:i], &msg); err != nil {
		t.log.Warnf("unmarshal message failed from node %s:%s: %s", nodename, conn.RemoteAddr(), err)
		return
	}
	t.cmdC <- hbctrl.CmdSetPeerSuccess{
		Nodename: msg.Nodename,
		HbID:     t.id,
		Success:  true,
	}
	t.msgC <- &msg
}

func newRx(ctx context.Context, name string, nodes map[string]string, addr, port, intf string, timeout time.Duration) *rx {
	id := name + ".rx"
	return &rx{
		ctx:     ctx,
		id:      id,
		nodes:   nodes,
		addr:    addr,
		port:    port,
		intf:    intf,
		timeout: timeout,
		log: plog.NewDefaultLogger().Attr("pkg", "daemon/hb/hbucast").
			Attr("hb_func", "rx").
			Attr("hb_name", name).
			Attr("hb_id", id).
			WithPrefix("daemon: hb: ucast: rx: " + name + ": "),
	}
}
