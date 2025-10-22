package hbucast

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/daemon/encryptconn"
	"github.com/opensvc/om3/daemon/hb/hbcrypto"
	"github.com/opensvc/om3/daemon/hb/hbctrl"
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

	msgMaxSize = 10000000 // max kind=full msg size

	// Create a new sync.Pool to manage the byte buffers. Used to reduce memory usage
	// during handling the messages.
	msgPool = sync.Pool{
		New: func() interface{} {
			// This creates a new byte slice of the specified size.
			return make([]byte, msgMaxSize)
		},
	}
)

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

func (t *rx) streamPeerDesc(addr string) string {
	addr, _, _ = strings.Cut(addr, ":")
	if len(t.addr) > 0 {
		if t.intf != "" {
			return fmt.Sprintf("%s:%s@%s ← %s", t.addr, t.port, t.intf, addr)
		} else {
			return fmt.Sprintf("%s:%s ← %s", t.addr, t.port, addr)
		}
	} else {
		if t.intf != "" {
			return fmt.Sprintf(":%s@%s ← %s", t.port, t.intf, addr)
		} else {
			return fmt.Sprintf(":%s ← %s", t.port, addr)
		}
	}
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

	listenConfig := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
			})
		},
	}

	var (
		listener net.Listener
		err      error
	)

	for i := 1; i < 5; i++ {
		listener, err = listenConfig.Listen(t.ctx, "tcp", t.addr+":"+t.port)
		if err != nil {
			if strings.Contains(err.Error(), "address already in use") {
				delay := time.Duration(int(time.Millisecond) * 100 * i)
				t.log.Infof("%s: retry in %s", err, delay)
				time.Sleep(delay)
				continue
			}
			t.log.Errorf("listen failed: %s", err)
			return err
		}
		break
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
				Desc:     t.streamPeerDesc(addr),
			}
			addr, _, _ := strings.Cut(addr, ":")
			addrs, err := resolver.LookupHost(ctx, addr)
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				t.log.Infof("add expected %s address: %s", node, addr)
				otherNodeIPM[addr] = struct{}{}
				otherNodeIPL = append(otherNodeIPL, addr)
			}
		}
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case <-ctx.Done():
				t.log.Infof("closing listener %s for %s", t.addr+":"+t.port, otherNodeIPL)
				_ = listener.Close()
				time.Sleep(100 * time.Millisecond)
				t.cancel()
				return
			}
		}()
		t.log.Infof("listen to %s for %s", t.addr+":"+t.port, otherNodeIPL)
		started <- true
		crypto := hbcrypto.CryptoFromContext(ctx)
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
			connAddr, _, err := net.SplitHostPort(conn.RemoteAddr().String())
			if err != nil {
				t.log.Warnf("%s", err)
				continue
			}
			if _, ok := otherNodeIPM[connAddr]; !ok {
				t.log.Warnf("unexpected connection from %s", connAddr)
				if err := conn.Close(); err != nil {
					t.log.Warnf("failed to close unexpected connection from %s: %s", connAddr, err)
				}
				continue
			}
			if err := conn.SetDeadline(time.Now().Add(messageTimeout)); err != nil {
				t.log.Infof("can't set read deadline for %s: %s", connAddr, err)
				continue
			}
			clearConn := encryptconn.New(conn, crypto.Load())
			wg.Add(1)
			go func() {
				defer wg.Done()
				t.handle(clearConn)
			}()
		}
		wg.Wait()
		t.log.Infof("stopped %s", t.addr)
	}()
	<-started
	t.log.Infof("started %s", t.addr)
	return nil
}

func (t *rx) handle(conn encryptconn.ConnNoder) {
	defer func() {
		if err := conn.Close(); err != nil {
			t.log.Warnf("unexpected error while closing connection from %s: %s", conn.RemoteAddr(), err)
		}
	}()
	data := msgPool.Get().([]byte)
	defer func() { msgPool.Put(data) }()
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
	cmdPeerSuccess := hbctrl.CmdSetPeerSuccess{
		Nodename: msg.Nodename,
		HbID:     t.id,
		Success:  true,
	}
	select {
	case <-t.ctx.Done():
		return
	case t.cmdC <- cmdPeerSuccess:
	}
	select {
	case <-t.ctx.Done():
	case t.msgC <- &msg:
	}
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
