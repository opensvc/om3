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
		localIPs    []net.IP
		lastNodeErr sync.Map

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

	// localIPs are the addresses the worker picks the source address it
	// dials from among. They are carried by the request because t.localIPs
	// is refreshed by the Start goroutine, which is the only one allowed to
	// read or write it.
	localIPs []net.IP
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
	if t.addr != "" {
		if t.intf != "" {
			return fmt.Sprintf("%s@%s → %s", t.addr, t.intf, addr)
		} else {
			return fmt.Sprintf("%s → %s", t.addr, addr)
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
		// connLocalIPs are the local addresses the connection source was
		// picked among, to detect a local ip change while it is
		// established
		var connLocalIPs []net.IP

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

				if conn != nil && !slices.EqualFunc(connLocalIPs, req.localIPs, net.IP.Equal) {
					// The local ips changed since we dialed: the
					// connection is bound to an address the node may
					// not own anymore, redial from the new ones.
					t.log.Infof("local ips changed from %s to %s, reconnect to %s", connLocalIPs, req.localIPs, addr)
					w.closeConn()
					conn = nil
				}

				if conn == nil {
					routes := t.routes(addr, req.localIPs)
					// Use a separate context for dial that respects t.ctx.
					// Cancel as soon as the dial returns: cancelling after a
					// successful dial doesn't affect the connection, and this
					// worker goroutine lives as long as the transmitter, so a
					// deferred cancel would pile up on its stack, one per
					// reconnect.
					dialCtx, dialCancel := context.WithTimeout(t.ctx, t.timeout)
					newConn, err := dialRoutes(dialCtx, routes)
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

		updateLocalIPs := func() {
			if localIPs, err := t.defaultLocalIPs(); err != nil {
				t.log.Errorf("%s", err)
			} else if !slices.EqualFunc(t.localIPs, localIPs, net.IP.Equal) {
				t.log.Infof("set local ips to %s", localIPs)
				t.localIPs = localIPs
			}
		}

		if localIPs, err := t.defaultLocalIPs(); err != nil {
			t.log.Errorf("%s", err)
		} else if len(localIPs) > 0 {
			t.log.Infof("set local ips to %s", localIPs)
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
				updateLocalIPs()
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

// defaultLocalIPs returns the addresses of the local nodename, among which
// the source address of a connection to a peer is picked, so rx on peer
// nodes see messages coming from an address they know the node by.
//
// The address configured for the heartbeat is the only one when set. The
// others are the ones the nodename resolves to, but a link-local, loopback or
// unspecified one: a peer knows the node by none of these, and a link-local
// one can't even be bound without its zone. A nodename the hosts file does
// not name resolves, through the myhostname nss module, to every address of
// the node, link-local ones first.
func (t *tx) defaultLocalIPs() ([]net.IP, error) {
	if t.addr != "" {
		return []net.IP{net.ParseIP(t.addr)}, nil
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(t.ctx, hostname.Hostname())
	if err != nil {
		return nil, fmt.Errorf("lookup sender addr: %s: %s", hostname.Hostname(), err)
	}
	l := make([]net.IP, 0, len(addrs))
	for _, addr := range addrs {
		if usableIP(addr.IP) {
			l = append(l, addr.IP)
		}
	}
	if len(l) == 0 {
		return nil, fmt.Errorf("lookup sender addr: %s: no usable address found", hostname.Hostname())
	}
	return l, nil
}

// usableIP says whether ip can be the address of one node for another.
func usableIP(ip net.IP) bool {
	return ip != nil && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsUnspecified()
}

// dialRoute is an address to dial a peer at, and the source address to dial
// it from, nil to leave it to the kernel.
type dialRoute struct {
	addr   string
	source net.IP
}

// dialRoutes dials the routes in turn, and returns the first connection
// made, or the error of the last route tried. They share the deadline of
// ctx, as the addresses of a name do when the dialer tries them.
func dialRoutes(ctx context.Context, routes []dialRoute) (net.Conn, error) {
	var err error
	for _, r := range routes {
		dialer := &net.Dialer{}
		if r.source != nil {
			dialer.LocalAddr = &net.TCPAddr{IP: r.source}
		}
		var conn net.Conn
		if conn, err = dialer.DialContext(ctx, "tcp", r.addr); err == nil {
			return conn, nil
		}
		if ctx.Err() != nil {
			break
		}
	}
	return nil, err
}

// routes returns the routes to the peer at addr, in the order to try them.
//
// The address configured for the heartbeat is the source of the only route.
// Otherwise the peer is resolved, and each of its addresses is dialed from a
// local address: first the ones a local address shares a subnet with, from
// that address, which is the one the kernel routes the peer through and the
// one the peers know the node by on that network, then the others, from a
// local address of their family, or from where the kernel routes them when
// none has it. A peer that can't be resolved is dialed as named.
func (t *tx) routes(addr string, localIPs []net.IP) []dialRoute {
	if t.addr != "" {
		return []dialRoute{{addr: addr, source: net.ParseIP(t.addr)}}
	}
	named := []dialRoute{{addr: addr}}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return named
	}
	peerAddrs, err := net.DefaultResolver.LookupIPAddr(t.ctx, host)
	if err != nil {
		return named
	}
	peerIPs := make([]net.IP, len(peerAddrs))
	for i, a := range peerAddrs {
		peerIPs[i] = a.IP
	}
	nets, _ := localNets()
	l := pickRoutes(peerIPs, localIPs, nets, port)
	if len(l) == 0 {
		return named
	}
	return l
}

// localNets returns the networks of the addresses of the node interfaces.
func localNets() ([]*net.IPNet, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	nets := make([]*net.IPNet, 0, len(addrs))
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok {
			nets = append(nets, ipNet)
		}
	}
	return nets, nil
}

// pickRoutes returns the routes to the usable peer addresses, as routes
// orders them.
func pickRoutes(peerIPs, localIPs []net.IP, nets []*net.IPNet, port string) []dialRoute {
	sameFamily := func(a, b net.IP) bool {
		return (a.To4() == nil) == (b.To4() == nil)
	}
	sameSubnet := func(local, peer net.IP) bool {
		for _, n := range nets {
			if n.IP.Equal(local) && n.Contains(peer) {
				return true
			}
		}
		return false
	}
	var subnet, others []dialRoute
	for _, peer := range peerIPs {
		if !usableIP(peer) {
			continue
		}
		r := dialRoute{addr: net.JoinHostPort(peer.String(), port)}
		for _, local := range localIPs {
			if sameFamily(local, peer) && sameSubnet(local, peer) {
				r.source = local
				break
			}
		}
		if r.source != nil {
			subnet = append(subnet, r)
			continue
		}
		for _, local := range localIPs {
			if sameFamily(local, peer) {
				r.source = local
				break
			}
		}
		others = append(others, r)
	}
	return append(subnet, others...)
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
