package hbucast

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/hbtype"
	"github.com/opensvc/om3/v3/daemon/hb/hbaudit"
	"github.com/opensvc/om3/v3/daemon/hb/hbctrl"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/stagger"
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
		localIPs    ipList
		lastNodeErr sync.Map

		// addrNotLocal are the addresses the name addr resolved to, none of
		// which the node holds, warned about once.
		addrNotLocal ipList

		name   string
		log    *plog.Logger
		cmdC   chan<- interface{}
		msgC   chan<- *hbtype.Msg
		cancel func()
		// Per-peer send workers, to serialize the sends to the same node.
		// Start creates one per configured node, before anything can use
		// them, and Stop closes them once the sender is done.
		sendWorkers map[string]*sendWorker
		// WaitGroup for send worker goroutines
		sendWorkersWG sync.WaitGroup
	}
)

// sendRequest holds data for a send operation
type sendRequest struct {
	data []byte

	// localIPs are the source addresses the worker dials from, the one
	// of the family of the peer address. They are carried by the request
	// because t.localIPs is refreshed by the Start goroutine, which is the
	// only one allowed to read or write it.
	localIPs ipList
}

// sendWorker serializes the sends to one peer node
type sendWorker struct {
	queue chan sendRequest

	// mu protects conn, which the worker goroutine owns, and Stop closes
	// to interrupt a write to a peer that stopped reading. Its deadline
	// would otherwise hold the shutdown for a whole timeout.
	mu   sync.Mutex
	conn net.Conn
}

// getConn returns the worker connection, nil when it has to dial one
func (w *sendWorker) getConn() net.Conn {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn
}

// setConn publishes the connection the worker just dialed
func (w *sendWorker) setConn(conn net.Conn) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.conn = conn
}

// closeConn closes the worker connection, if it has one. The worker calls
// it when a send fails, and Stop to interrupt a blocked send.
func (w *sendWorker) closeConn() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.conn != nil {
		_ = w.conn.Close()
		w.conn = nil
	}
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
	// Close the queues to unblock the workers, and their connections: a
	// worker can be parked in a write to a peer that stopped reading, and
	// only its own deadline, a timeout away, would end it.
	for node, w := range t.sendWorkers {
		delete(t.sendWorkers, node)
		close(w.queue)
		w.closeConn()
	}
	t.sendWorkersWG.Wait()
	t.log.Tracef("wait done")
	return nil
}

func (t *tx) Ctx() context.Context {
	return t.ctx
}

func (t *tx) streamPeerDesc(addr string) string {
	if len(t.localIPs) > 0 {
		if t.intf != "" {
			return fmt.Sprintf("%s@%s → %s", t.localIPs, t.intf, addr)
		} else {
			return fmt.Sprintf("%s → %s", t.localIPs, addr)
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
// Must be called from the Start goroutine: it reads t.localIPs.
func (t *tx) sendToNode(node string, b []byte) {
	w, ok := t.sendWorkers[node]
	if !ok {
		// can't happen: Start creates a worker per configured node
		t.log.Warnf("no send worker for node %s", node)
		return
	}

	// Try to send without blocking first (non-blocking send)
	select {
	case w.queue <- sendRequest{data: b, localIPs: t.localIPs}:
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
func (t *tx) startSendWorker(node, addr string, w *sendWorker) {
	t.sendWorkersWG.Add(1)
	go func() {
		defer t.sendWorkersWG.Done()
		// The worker connection is closed on every exit path, and by Stop
		// when it has to interrupt a send.
		defer w.closeConn()
		// connLocalIPs are the source addresses the connection was dialed
		// from, to detect a local ip change while it is established
		var connLocalIPs ipList

		for {
			select {
			case <-t.ctx.Done():
				// Context cancelled, exit
				return
			case req, ok := <-w.queue:
				if !ok {
					// Queue closed, exit
					return
				}

				conn := w.getConn()

				if conn != nil && !connLocalIPs.Equal(req.localIPs) {
					// The local ip changed since we dialed: the
					// connection is bound to an address the node may
					// not own anymore, redial from the new one.
					t.log.Infof("local ip changed from %s to %s, reconnect to %s", connLocalIPs, req.localIPs, addr)
					w.closeConn()
					conn = nil
				}

				if conn == nil {
					// Use a separate context for dial that respects t.ctx.
					// Cancel as soon as the dial returns: cancelling after a
					// successful dial doesn't affect the connection, and this
					// worker goroutine lives as long as the transmitter, so a
					// deferred cancel would pile up on its stack, one per
					// reconnect.
					dialCtx, dialCancel := context.WithTimeout(t.ctx, t.timeout)
					newConn, err := dialPeer(dialCtx, t.timeout, req.localIPs, addr)
					dialCancel()
					if err != nil {
						t.handleSendError(node, err)
						continue
					}
					conn = newConn
					connLocalIPs = req.localIPs
					w.setConn(conn)
				}

				// Set deadline on the connection
				if err := conn.SetDeadline(time.Now().Add(t.timeout)); err != nil {
					t.handleSendError(node, err)
					w.closeConn()
					continue
				}

				// Send the data, already null terminated by the sender
				if n, err := conn.Write(req.data); err != nil {
					t.log.Tracef("write failed to %s: %v (wrote %d/%d bytes)", addr, err, n, len(req.data))
					t.handleSendError(node, err)
					w.closeConn()
				} else if n != len(req.data) {
					t.log.Tracef("short write to %s: %d/%d bytes", addr, n, len(req.data))
					t.handleSendError(node, fmt.Errorf("short write: %d/%d", n, len(req.data)))
					w.closeConn()
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

	auditName := strings.Replace(t.id, "hb#", "hb:", 1)
	hbaudit.EnableAudit(ctx, t.id, t.log, "hb", auditName, strings.TrimSuffix(auditName, ".tx"))

	// One worker per peer node, created before the sender can reach them,
	// so the map is never written again.
	t.sendWorkers = make(map[string]*sendWorker, len(t.nodes))
	for node, addr := range t.nodes {
		w := &sendWorker{queue: make(chan sendRequest, 1)}
		t.sendWorkers[node] = w
		t.startSendWorker(node, addr, w)
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
			if localIPs, err := t.defaultLocalIPs(); err != nil {
				t.log.Errorf("%s", err)
			} else if !t.localIPs.Equal(localIPs) {
				if len(localIPs) == 0 {
					t.log.Infof("undetermined local ip")
				} else {
					t.log.Infof("set local ip to %s", localIPs)
				}
				t.localIPs = localIPs
			}
		}

		if localIPs, err := t.defaultLocalIPs(); err != nil {
			t.log.Errorf("%s", err)
		} else if len(localIPs) > 0 {
			t.log.Infof("set local ip to %s", localIPs)
			t.localIPs = localIPs
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

// ipList is the source addresses a heartbeat is sent from, in the order
// they are preferred.
type ipList []net.IP

// Equal says l and other hold the same addresses in the same order.
func (l ipList) Equal(other ipList) bool {
	return slices.EqualFunc(l, other, func(a, b net.IP) bool { return a.Equal(b) })
}

func (l ipList) String() string {
	s := make([]string, len(l))
	for i, ip := range l {
		s[i] = ip.String()
	}
	return strings.Join(s, ",")
}

// defaultLocalIPs returns the source addresses of the heartbeat, so rx on
// peer nodes see messages coming from a known cluster member: the address
// of addr, or the first address of the local nodename.
//
// An addr given by its name is resolved, as the nodename is, and the
// addresses it resolves to that the node holds are the ones sent from, the
// one of the family of each peer. It was sent from no address in
// particular, so from the one the kernel picks, as a temporary IPv6
// address, which the rx of the peers, accepting the addresses the name of
// the node resolves to, refused. A name resolving to no address the node
// holds is sent from the address the kernel picks, as before, with a
// warning.
func (t *tx) defaultLocalIPs() (ipList, error) {
	if t.addr != "" {
		if ip := net.ParseIP(t.addr); ip != nil {
			return ipList{ip}, nil
		}
		return t.addrLocalIPs()
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(t.ctx, hostname.Hostname())
	if err != nil {
		return nil, fmt.Errorf("lookup sender addr: %s: %s", t.addr, err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("lookup sender addr: %s: no address found ", t.addr)
	}
	return ipList{addrs[0].IP}, nil
}

// addrLocalIPs returns the addresses the name addr resolves to that the node
// holds, in the order of the resolver.
func (t *tx) addrLocalIPs() (ipList, error) {
	addrs, err := lookupIPAddr(t.ctx, t.addr)
	if err != nil {
		return nil, fmt.Errorf("lookup sender addr: %s: %s", t.addr, err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("lookup sender addr: %s: no address found", t.addr)
	}
	var resolved, held ipList
	for _, a := range addrs {
		resolved = append(resolved, a.IP)
		if local, err := isLocalIP(a.IP); err != nil {
			return nil, fmt.Errorf("lookup sender addr: %s: %s", t.addr, err)
		} else if local {
			held = append(held, a.IP)
		}
	}
	if len(held) == 0 {
		if !t.addrNotLocal.Equal(resolved) {
			t.log.Warnf("sender addr %s resolves to %s, which the node does not hold: send from the address the kernel picks", t.addr, resolved)
			t.addrNotLocal = resolved
		}
		return nil, nil
	}
	t.addrNotLocal = nil
	return held, nil
}

// peerAttemptDelay is how long the connect to an address of a peer runs
// before the connect to its next address starts too.
var peerAttemptDelay = 300 * time.Millisecond

// dialPeer connects to addr from the address of localIPs of the family of a
// peer address, the addresses of the peer tried in the order of the resolver,
// each one starting when the previous one failed or ran peerAttemptDelay:
// an address that does not answer does not hold the others for the whole
// timeout. With no source, or none of the family of a peer address, as an
// IPv6 source for a peer reached over IPv4 only, it connects from the
// address the kernel picks: a source of another family can not reach it.
func dialPeer(ctx context.Context, timeout time.Duration, localIPs ipList, addr string) (net.Conn, error) {
	unbound := func() (net.Conn, error) {
		return (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp", addr)
	}
	if len(localIPs) == 0 {
		return unbound()
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	var peerIPs []net.IP
	if ip := net.ParseIP(host); ip != nil {
		peerIPs = []net.IP{ip}
	} else {
		addrs, err := lookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, a := range addrs {
			peerIPs = append(peerIPs, a.IP)
		}
	}
	var attempts []stagger.Attempt
	for _, peer := range peerIPs {
		i := slices.IndexFunc(localIPs, func(local net.IP) bool { return sameFamily(local, peer) })
		if i < 0 {
			continue
		}
		d := &net.Dialer{Timeout: timeout, LocalAddr: &net.TCPAddr{IP: localIPs[i]}}
		peerAddr := net.JoinHostPort(peer.String(), port)
		attempts = append(attempts, func(ctx context.Context) (net.Conn, error) {
			return d.DialContext(ctx, "tcp", peerAddr)
		})
	}
	if len(attempts) == 0 {
		return unbound()
	}
	return stagger.Dial(ctx, peerAttemptDelay, attempts)
}

// sameFamily says a and b are both IPv4, or both IPv6.
func sameFamily(a, b net.IP) bool {
	return (a.To4() != nil) == (b.To4() != nil)
}

var (
	// lookupIPAddr resolves the name of an address.
	lookupIPAddr = net.DefaultResolver.LookupIPAddr

	// interfaceAddrs returns the addresses of the node.
	interfaceAddrs = net.InterfaceAddrs
)

// isLocalIP says the node holds ip.
func isLocalIP(ip net.IP) (bool, error) {
	addrs, err := interfaceAddrs()
	if err != nil {
		return false, err
	}
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && n.IP.Equal(ip) {
			return true, nil
		}
	}
	return false, nil
}

// handleSendError handles send errors with deduplication logging
func (t *tx) handleSendError(node string, err error) {
	if t.ctx.Err() != nil {
		// stopping: the error is the dial or the send Stop just interrupted
		return
	}
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
