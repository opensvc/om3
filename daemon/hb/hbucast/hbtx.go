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

		name   string
		log    *plog.Logger
		cmdC   chan<- interface{}
		msgC   chan<- *hbtype.Msg
		cancel func()
		// Per-peer send queues to serialize sends to the same node. Start
		// creates one per configured node, before anything can use them,
		// and Stop closes them once the sender is done.
		sendQueues map[string]chan sendRequest
		// WaitGroup for send worker goroutines
		sendWorkers sync.WaitGroup
	}
)

// sendRequest holds data for a send operation
type sendRequest struct {
	data []byte

	// localIP is the source address the worker must dial from. It is
	// carried by the request because t.localIP is refreshed by the Start
	// goroutine, which is the only one allowed to read or write it.
	localIP net.IP
}

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
	// Wait for the Start goroutine first: it is the only sendToNode caller,
	// so the send queues have no writer left once it is done. Closing them
	// before would risk a send on a closed channel.
	t.Wait()
	// Close all send queues to unblock workers
	for node, queue := range t.sendQueues {
		delete(t.sendQueues, node)
		close(queue)
	}
	t.sendWorkers.Wait()
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

// sendToNode queues a send request for a specific node
//
// Must be called from the Start goroutine: it reads t.localIP.
func (t *tx) sendToNode(node string, b []byte) {
	queue, ok := t.sendQueues[node]
	if !ok {
		// can't happen: Start creates a queue per configured node
		t.log.Warnf("no send queue for node %s", node)
		return
	}

	// Try to send without blocking first (non-blocking send)
	select {
	case queue <- sendRequest{data: b, localIP: t.localIP}:
		// Successfully queued
	default:
		// Queue is full, drop the message to avoid blocking
		// This means a send is already in progress and we don't want to stack up
		t.log.Tracef("send queue full for node %s, dropping message", node)
	}
}

// startSendWorker starts the goroutine serializing the sends to a peer
// node. It maintains its own connection, redialing when a send fails or
// the local ip changes, and exits when the transmitter context is done or
// the queue is closed.
func (t *tx) startSendWorker(node, addr string, q chan sendRequest) {
	t.sendWorkers.Add(1)
	go func() {
		defer t.sendWorkers.Done()
		// Worker maintains its own persistent connection
		var conn net.Conn
		// connLocalIP is the source address conn is bound to, to
		// detect a local ip change while conn is established
		var connLocalIP net.IP

		for {
			select {
			case <-t.ctx.Done():
				// Context cancelled, close connection and exit
				if conn != nil {
					conn.Close()
					conn = nil
				}
				return
			case req, ok := <-q:
				if !ok {
					// Queue closed, exit
					if conn != nil {
						conn.Close()
					}
					return
				}

				if conn != nil && !connLocalIP.Equal(req.localIP) {
					// The local ip changed since we dialed: the
					// connection is bound to an address the node may
					// not own anymore, redial from the new one.
					t.log.Infof("local ip changed from %s to %s, reconnect to %s", connLocalIP, req.localIP, addr)
					conn.Close()
					conn = nil
				}

				if conn == nil {
					// Create new connection with context-aware dialer
					localAddr := net.TCPAddr{
						IP:   req.localIP,
						Port: 0,
					}
					dialer := &net.Dialer{
						Timeout:   t.timeout,
						LocalAddr: &localAddr,
					}
					// Use a separate context for dial that respects t.ctx.
					// Cancel as soon as the dial returns: cancelling after a
					// successful dial doesn't affect the connection, and this
					// worker goroutine lives as long as the transmitter, so a
					// deferred cancel would pile up on its stack, one per
					// reconnect.
					dialCtx, dialCancel := context.WithTimeout(t.ctx, t.timeout)
					newConn, err := dialer.DialContext(dialCtx, "tcp", addr)
					dialCancel()
					if err != nil {
						t.handleSendError(node, err)
						continue
					}
					conn = newConn
					connLocalIP = req.localIP
				}

				// Set deadline on the connection
				if err := conn.SetDeadline(time.Now().Add(t.timeout)); err != nil {
					t.handleSendError(node, err)
					conn.Close()
					conn = nil
					continue
				}

				// Send the data, already null terminated by the sender
				if n, err := conn.Write(req.data); err != nil {
					t.log.Tracef("write failed to %s: %v (wrote %d/%d bytes)", addr, err, n, len(req.data))
					t.handleSendError(node, err)
					conn.Close()
					conn = nil
				} else if n != len(req.data) {
					t.log.Tracef("short write to %s: %d/%d bytes", addr, n, len(req.data))
					t.handleSendError(node, fmt.Errorf("short write: %d/%d", n, len(req.data)))
					conn.Close()
					conn = nil
				} else {
					t.log.Tracef("sent %d bytes to %s", len(req.data), addr)
					t.clearDedupLog(node)
					// Send success notification, but don't block on it
					select {
					case t.cmdC <- hbctrl.CmdSetPeerSuccess{
						Nodename: node,
						HbID:     t.id,
						Success:  true,
					}:
					case <-t.ctx.Done():
						// Context cancelled, skip notification
						if conn != nil {
							conn.Close()
							conn = nil
						}
						return
					}

					// Reset deadline for next write (connection stays open)
					if err := conn.SetDeadline(time.Now().Add(t.timeout)); err != nil {
						t.log.Tracef("failed to reset deadline for %s: %v", addr, err)
						// Continue with connection, it might still work
					}
				}
			}
		}
	}()
}

// Start implements the Start function of Transmitter interface for tx
func (t *tx) Start(cmdC chan<- interface{}, msgC <-chan []byte) error {
	started := make(chan bool)
	ctx, cancel := context.WithCancel(t.ctx)
	t.ctx = ctx
	t.cancel = cancel
	t.cmdC = cmdC
	t.Add(1)
	hbaudit.EnableAudit(ctx, t.id, t.log, "hb", strings.Replace(t.id, "hb#", "hb:", 1))

	// One queue and one worker per peer node, created before the sender
	// can reach them, so the map is never written again.
	t.sendQueues = make(map[string]chan sendRequest, len(t.nodes))
	for node, addr := range t.nodes {
		queue := make(chan sendRequest, 1)
		t.sendQueues[node] = queue
		t.startSendWorker(node, addr, queue)
	}

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
				// The extra byte is the null frame terminator the peer rx
				// scans for, left at zero by make. Framing the message here
				// keeps the buffer the workers share read-only for them.
				protectedB := make([]byte, len(b)+1)
				copy(protectedB, b)
				for node := range t.nodes {
					t.sendToNode(node, protectedB)
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
