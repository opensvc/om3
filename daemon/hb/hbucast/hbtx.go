package hbucast

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/hbtype"
	"github.com/opensvc/om3/v3/daemon/hb/hbaudit"
	"github.com/opensvc/om3/v3/daemon/hb/hbctrl"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	// tx holds a hb unicast transmitter
	tx struct {
		sync.WaitGroup
		ctx         context.Context
		id          string
		nodes       map[string]string
		addr        string
		port        string
		intf        string
		interval    time.Duration
		timeout     time.Duration
		localIP     net.IP
		lastNodeErr sync.Map

		name      string
		log       *plog.Logger
		cmdC      chan<- interface{}
		msgC      chan<- *hbtype.Msg
		cancel    func()
		// Per-peer connections that are kept open
		peerConns map[string]net.Conn
		peerMutex sync.Mutex
	}
)

// ID implements the ID function of Transmitter interface for tx
func (t *tx) ID() string {
	return t.id
}

// Stop implements the Stop function of Transmitter interface for tx
func (t *tx) Stop() error {
	t.log.Tracef("cancelling")
	t.cancel()
	for node := range t.nodes {
		t.cmdC <- hbctrl.CmdDelWatcher{
			HbID:     t.id,
			Nodename: node,
		}
	}
	// Close all peer connections
	t.peerMutex.Lock()
	for _, conn := range t.peerConns {
		conn.Close()
	}
	t.peerConns = make(map[string]net.Conn)
	t.peerMutex.Unlock()
	t.Wait()
	t.log.Tracef("wait done")
	return nil
}

func (t *tx) Ctx() context.Context {
	return t.ctx
}

func (t *tx) streamPeerDesc(addr string) string {
	if len(t.localIP) > 0 {
		if t.intf != "" {
			return fmt.Sprintf("%s@%s → %s", t.localIP, t.intf, addr)
		} else {
			return fmt.Sprintf("%s → %s", t.localIP, addr)
		}
	} else {
		if t.intf != "" {
			return fmt.Sprintf("@%s → %s", t.intf, addr)
		} else {
			return fmt.Sprintf("→ %s", addr)
		}
	}
}

// Start implements the Start function of Transmitter interface for tx
func (t *tx) Start(cmdC chan<- interface{}, msgC <-chan []byte) error {
	started := make(chan bool)
	ctx, cancel := context.WithCancel(t.ctx)
	t.ctx = ctx
	t.cancel = cancel
	t.cmdC = cmdC
	t.peerConns = make(map[string]net.Conn)
	t.Add(1)
	hbaudit.EnableAudit(ctx, t.id, t.log, "hb", strings.Replace(t.id, "hb#", "hb:", 1))

	go func() {
		defer t.Done()
		t.log.Infof("starting: timeout %s, interval: %s", t.timeout, t.interval)
		for node, addr := range t.nodes {
			cmdC <- hbctrl.CmdAddWatcher{
				HbID:     t.id,
				Nodename: node,
				Ctx:      ctx,
				Timeout:  t.timeout,
				Desc:     t.streamPeerDesc(addr),
			}
		}
		started <- true
		var b []byte

		sendTicker := time.NewTicker(t.interval)
		defer sendTicker.Stop()

		localIPTicker := time.NewTicker(30 * time.Second)
		defer localIPTicker.Stop()

		updateLocalIP := func() {
			if localIP, err := t.defaultLocalIP(); err != nil {
				t.log.Errorf("%s", err)
			} else if !t.localIP.Equal(localIP) {
				t.log.Infof("set local ip to %s", localIP)
				t.localIP = localIP
			}
		}

		if localIP, err := t.defaultLocalIP(); err != nil {
			t.log.Errorf("%s", err)
		} else if localIP != nil {
			t.log.Infof("set local ip to %s", localIP)
			t.localIP = localIP
		} else {
			t.log.Infof("undetermined local ip")
		}

		var reason string
		for {
			select {
			case <-ctx.Done():
				t.log.Infof("stopped")
				return
			case b = <-msgC:
				reason = "send msg"
				sendTicker.Reset(t.interval)
			case <-sendTicker.C:
				reason = "send msg (interval)"
			case <-localIPTicker.C:
				updateLocalIP()
			}
			if len(b) == 0 {
				continue
			} else {
				t.log.Tracef(reason)
				protectedB := make([]byte, len(b))
				copy(protectedB, b)
				for node, addr := range t.nodes {
					go t.send(node, addr, protectedB)
				}
			}
		}
	}()
	<-started
	t.log.Infof("started")
	return nil
}

// defaultLocalIP returns the ip address of the local nodename, so rx on peer
// nodes see messages coming from a known cluster member.
func (t *tx) defaultLocalIP() (net.IP, error) {
	if t.addr != "" {
		return net.ParseIP(t.addr), nil
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(t.ctx, hostname.Hostname())
	if err != nil {
		return nil, fmt.Errorf("lookup sender addr: %s: %s", t.addr, err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("lookup sender addr: %s: no address found ", t.addr)
	}
	return addrs[0].IP, nil
}

func (t *tx) send(node, addr string, b []byte) {
	t.log.Tracef("send: starting for node %s, addr %s, data len %d", node, addr, len(b))
	localAddr := net.TCPAddr{
		IP:   t.localIP,
		Port: 0,
	}

	// Get or create persistent connection for this peer
	rawConn, err := t.getPeerConn(node, addr, localAddr)
	if err != nil {
		t.handleSendError(node, err)
		return
	}

	// Set deadline on the raw connection
	if err := rawConn.SetDeadline(time.Now().Add(t.timeout)); err != nil {
		t.handleSendError(node, err)
		return
	}

	// Create encryptconn wrapper to add null terminator
	// The data in b is already encrypted by the main hb code, but we need to add
	// the null terminator that encryptconn.ReadWithNode expects
	// However, b is already the final encrypted message, so we need to write it
	// with the null terminator
	// For now, just write b + null byte directly
	dataWithTerminator := append(b, 0x00)
	
	t.log.Tracef("sending %d bytes (+%d terminator) to %s", len(b), 1, addr)
	if n, err := rawConn.Write(dataWithTerminator); err != nil {
		t.log.Tracef("write failed to %s: %v (wrote %d/%d bytes)", addr, err, n, len(dataWithTerminator))
		t.handleSendError(node, err)
		// Remove the connection so we can reconnect next time
		t.removePeerConn(node)
		return
	} else if n != len(dataWithTerminator) {
		t.log.Tracef("short write to %s: %d/%d bytes", addr, n, len(dataWithTerminator))
		t.handleSendError(node, fmt.Errorf("short write: %d/%d", n, len(dataWithTerminator)))
		// Remove the connection so we can reconnect next time
		t.removePeerConn(node)
		return
	}

	t.log.Tracef("successfully sent %d bytes to %s", len(b), addr)
	t.clearDedupLog(node)

	t.cmdC <- hbctrl.CmdSetPeerSuccess{
		Nodename: node,
		HbID:     t.id,
		Success:  true,
	}
}

// getPeerConn returns the persistent connection for a peer, creating it if necessary
func (t *tx) getPeerConn(node, addr string, localAddr net.TCPAddr) (net.Conn, error) {
	t.peerMutex.Lock()
	defer t.peerMutex.Unlock()
	
	if conn, exists := t.peerConns[node]; exists {
		return conn, nil
	}
	
	// Create new connection
	dialer := net.Dialer{
		Timeout:   t.timeout,
		LocalAddr: &localAddr,
	}
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	
	t.peerConns[node] = conn
	t.log.Tracef("created new persistent connection to %s (%s)", node, addr)
	return conn, nil
}

// removePeerConn removes a peer connection from the map
func (t *tx) removePeerConn(node string) {
	t.peerMutex.Lock()
	defer t.peerMutex.Unlock()
	if conn, exists := t.peerConns[node]; exists {
		conn.Close()
		delete(t.peerConns, node)
		t.log.Tracef("removed connection to %s", node)
	}
}

// handleSendError handles send errors with deduplication logging
func (t *tx) handleSendError(node string, err error) {
	newErr := err.Error()
	if lastErr, ok := t.lastNodeErr.Load(node); ok {
		if lastErr == newErr {
			return
		} else if lastErr != "" {
			t.log.Infof("end a send error period for node %s: %s", node, lastErr)
		}
	}
	if newErr != "" {
		t.log.Warnf("begin a send error period for node %s: %s", node, newErr)
		t.lastNodeErr.Store(node, newErr)
	} else {
		t.lastNodeErr.Delete(node)
	}
}

// clearDedupLog clears the deduplication log for a node
func (t *tx) clearDedupLog(node string) {
	if lastErr, ok := t.lastNodeErr.Load(node); !ok {
		return
	} else {
		t.log.Infof("end a send error period for node %s: %s", node, lastErr)
		t.lastNodeErr.Delete(node)
	}
}

func newTx(ctx context.Context, name string, nodes map[string]string, addr, port, intf string, timeout, interval time.Duration) *tx {
	id := name + ".tx"
	return &tx{
		ctx:      ctx,
		id:       id,
		nodes:    nodes,
		addr:     addr,
		port:     port,
		intf:     intf,
		interval: interval,
		timeout:  timeout,
		log: plog.NewDefaultLogger().Attr("pkg", "daemon/hb/hbucast").
			Attr("hb_func", "tx").
			Attr("hb_name", name).
			Attr("hb_id", id).
			WithPrefix("daemon: hb: ucast: tx: " + name + ": "),
	}
}
