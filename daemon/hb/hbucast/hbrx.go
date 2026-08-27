package hbucast

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/opensvc/om3/v3/core/hbtype"
	"github.com/opensvc/om3/v3/daemon/encryptconn"
	"github.com/opensvc/om3/v3/daemon/hb/hbaudit"
	"github.com/opensvc/om3/v3/daemon/hb/hbcrypto"
	"github.com/opensvc/om3/v3/daemon/hb/hbctrl"
	"github.com/opensvc/om3/v3/util/plog"
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

		// Track current connection per peer (peerAddr -> encryptconn.ConnNoder)
		// Accept loop is the only writer; handlers only read their own connection
		peerConns sync.Map
	}
)

var (
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
	t.log.Tracef("cancelling")
	t.cancel()
	for node := range t.nodes {
		t.cmdC <- hbctrl.CmdDelWatcher{
			HbID:     t.id,
			Nodename: node,
		}
	}
	// Note: the active connections are closed by the accept loop when it
	// stops, so a handler blocked in ReadWithNode doesn't hold the shutdown
	// until its read deadline expires.
	t.Wait()
	t.log.Tracef("wait done")
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
	t.ctx = ctx

	hbaudit.EnableAudit(ctx, t.id, t.log, "hb", strings.Replace(t.id, "hb#", "hb:", 1))

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
				t.log.Debugf("%s: retry in %s", err, delay)
				time.Sleep(delay)
				continue
			}
			cancel()
			return err
		}
		break
	}

	if listener == nil {
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
		// Decrypt through a loader, not through a snapshot of the crypto:
		// a connection outlives heartbeat secret rotations.
		crypto := hbcrypto.LoaderFromContext(ctx)
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
				conn.Close()
				continue
			}
			if _, ok := otherNodeIPM[connAddr]; !ok {
				t.log.Warnf("unexpected connection from %s", connAddr)
				conn.Close()
				continue
			}
			clearConn := encryptconn.New(conn, crypto)
			// Check if we already have a handler for this peer and close its connection
			// This is done atomically in the accept loop before starting new handler
			if oldConnI, hasOld := t.peerConns.Load(connAddr); hasOld {
				oldConn := oldConnI.(encryptconn.ConnNoder)
				t.log.Tracef("replacing existing connection from %s with new one", connAddr)
				// Close old connection; its handler will exit on next read with error
				oldConn.Close()
				t.peerConns.Delete(connAddr)
			}
			// Store new connection before starting handler to prevent race
			t.peerConns.Store(connAddr, clearConn)
			wg.Add(1)
			go func(peerAddr string, c encryptconn.ConnNoder) {
				defer wg.Done()
				defer c.Close()
				// Do NOT touch peerConns map - only accept loop manages it
				t.handleLoop(c, peerAddr)
			}(connAddr, clearConn)
		}
		// The accept loop is the only writer of peerConns and it is done:
		// no new connection can show up. Close the ones still tracked to
		// unblock their handler, which would otherwise sit in ReadWithNode
		// until its read deadline expires.
		t.peerConns.Range(func(key, value any) bool {
			t.peerConns.Delete(key)
			_ = value.(encryptconn.ConnNoder).Close()
			return true
		})
		wg.Wait()
		t.log.Infof("stopped %s", t.addr)
	}()
	<-started
	t.log.Infof("started %s", t.addr)
	return nil
}

func (t *rx) handleLoop(conn encryptconn.ConnNoder, peerAddr string) {
	// Set a generous read deadline to prevent idle connections from blocking indefinitely.
	// This is long enough to cover the heartbeat interval (default 5s) with some margin.
	// The deadline will be reset before each read operation.
	deadline := t.timeout * 3
	if deadline < 10*time.Second {
		deadline = 10 * time.Second
	}
	t.log.Tracef("starting to read messages from %s", peerAddr)
	data := msgPool.Get().([]byte)
	defer func() { msgPool.Put(data) }()

	msgCount := 0
	for {
		// Check context before blocking on read
		select {
		case <-t.ctx.Done():
			t.log.Tracef("context cancelled, stopping after %d messages from %s", msgCount, peerAddr)
			return
		default:
		}

		// Set read deadline for this iteration
		if err := conn.SetReadDeadline(time.Now().Add(deadline)); err != nil {
			t.log.Warnf("failed to set read deadline for %s: %v", peerAddr, err)
			return
		}

		// Reset buffer for each message
		buffer := data[:cap(data)]

		// Read will block until data arrives, connection is closed, or deadline is reached
		i, nodename, err := conn.ReadWithNode(buffer)
		if err != nil {
			switch {
			case errors.Is(err, io.EOF):
				// the peer closed the connection, it will reconnect
				t.log.Tracef("EOF from %s after %d messages", peerAddr, msgCount)
			case errors.Is(err, net.ErrClosed):
				// we closed the connection: peer reconnect, or stop
				t.log.Tracef("connection from %s closed after %d messages", peerAddr, msgCount)
			case errors.Is(err, os.ErrDeadlineExceeded):
				t.log.Warnf("no message from %s for %s, closing the connection", peerAddr, deadline)
			default:
				t.log.Warnf("read from %s failed after %d messages: %s", peerAddr, msgCount, err)
			}
			return
		}
		msgCount++

		if i >= (msgMaxSize - 10000) {
			t.log.Warnf("read huge message from node %s:%s msg size: %d", nodename, peerAddr, i)
		}
		msg := hbtype.Msg{}
		if err := json.Unmarshal(buffer[:i], &msg); err != nil {
			t.log.Warnf("unmarshal message failed from node %s:%s: %s", nodename, peerAddr, err)
			return
		}
		t.log.Tracef("read %d bytes from node %s (kind=%s, msg #%d)", i, nodename, msg.Kind, msgCount)

		cmdPeerSuccess := hbctrl.CmdSetPeerSuccess{
			Nodename: msg.Nodename,
			HbID:     t.id,
			Success:  true,
		}
		select {
		case <-t.ctx.Done():
			t.log.Tracef("context done, stopping after %d messages", msgCount)
			return
		case t.cmdC <- cmdPeerSuccess:
		}
		select {
		case <-t.ctx.Done():
			t.log.Tracef("context done while sending msg, stopping after %d messages", msgCount)
			return
		case t.msgC <- &msg:
		}
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
