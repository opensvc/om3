// Package console serves the console sessions of a node.
//
// The daemon listens on the console port and hands every connection it
// accepts to a process of its own, which does all the rest: the TLS
// handshake, the verification of the ticket, the websocket, and the command
// run in a terminal. The daemon never reads a byte of a session, and a
// session outlives it: a daemon restart does not interrupt the consoles.
package console

import (
	"crypto/rsa"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/websocket"

	"github.com/opensvc/om3/v3/core/console"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	// Config is what a session process needs of the node it runs on.
	Config struct {
		// Hostname is the name of this node: a ticket for another node
		// is relayed to it.
		Hostname string

		// Certificate is what the session authenticates the node to the
		// client with.
		Certificate tls.Certificate

		// VerifyKey is the public key of the key the tickets are signed
		// with.
		VerifyKey *rsa.PublicKey

		// TicketDir is where the used tickets are marked, and SessionDir
		// where the sessions are recorded.
		TicketDir  string
		SessionDir string

		// Command returns the command a session runs in its terminal.
		Command func(console.Target) ([]string, error)

		// PeerURL returns the url of the console endpoint of a peer
		// node, and PeerTLSConfig how to verify it.
		PeerURL       func(node string) (string, error)
		PeerTLSConfig *tls.Config

		// Authenticated is called once the ticket of the session is
		// accepted, to tell the daemon the connection is no longer one
		// of the unauthenticated connections it bounds the number of.
		Authenticated func()

		Log *plog.Logger
	}

	// oneConnListener hands one connection to a http server, and nothing
	// after it until it is closed.
	oneConnListener struct {
		conn net.Conn
		once sync.Once
		done chan struct{}
		stop sync.Once
	}

	// wsWriter serializes the writes to a websocket, which takes one
	// writer at a time.
	wsWriter struct {
		sync.Mutex
		conn *websocket.Conn
	}
)

const (
	// handshakeTimeout bounds the time a connection has to present a
	// valid ticket: a connection costs a process from the moment it is
	// accepted.
	handshakeTimeout = 10 * time.Second

	writeTimeout = 10 * time.Second

	// pingInterval is how often the client is pinged. The pings keep a
	// quiet session alive through the proxies that drop idle
	// connections, and a client that stops answering them is gone.
	pingInterval = 25 * time.Second
	pongTimeout  = 3 * pingInterval

	// killGrace is how long the command is given to end after its
	// terminal was hung up, before it is killed.
	killGrace = 5 * time.Second
)

var upgrader = websocket.Upgrader{
	HandshakeTimeout: handshakeTimeout,
	// The ticket is what authorizes a session, and a page of another
	// origin does not have it: the webapp is served by the api listener,
	// which is another origin than this one.
	CheckOrigin: func(*http.Request) bool { return true },
}

func (t *oneConnListener) Accept() (net.Conn, error) {
	var conn net.Conn
	t.once.Do(func() { conn = t.conn })
	if conn != nil {
		return conn, nil
	}
	<-t.done
	return nil, net.ErrClosed
}

func (t *oneConnListener) Close() error {
	t.stop.Do(func() { close(t.done) })
	return nil
}

func (t *oneConnListener) Addr() net.Addr {
	return t.conn.LocalAddr()
}

func (t *wsWriter) write(messageType int, b []byte) error {
	t.Lock()
	defer t.Unlock()
	_ = t.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	return t.conn.WriteMessage(messageType, b)
}

// end tells the client why the session ends, and closes the websocket.
func (t *wsWriter) end(m console.Message) {
	m.Type = console.MsgExit
	_ = t.write(websocket.TextMessage, m.Encode())
	t.Lock()
	defer t.Unlock()
	_ = t.conn.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, m.Reason),
		time.Now().Add(writeTimeout))
}

// Serve serves the console session of one connection, until it ends.
func Serve(conn net.Conn, cfg Config) error {
	_ = conn.SetDeadline(time.Now().Add(handshakeTimeout))
	tlsConn := tls.Server(conn, &tls.Config{
		Certificates: []tls.Certificate{cfg.Certificate},
		MinVersion:   tls.VersionTLS12,
		// A websocket is opened by a http/1.1 upgrade.
		NextProtos: []string{"http/1.1"},
	})
	listener := &oneConnListener{conn: tlsConn, done: make(chan struct{})}
	var hijacked atomic.Bool
	server := &http.Server{
		ReadHeaderTimeout: handshakeTimeout,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cfg.handle(w, r, conn)
			// A session took the connection from the server, and is over
			// when this returns. An answer that is no session is not sent
			// yet: the server writes it once this returns, then closes the
			// connection, which is when the process may end. Ending it
			// here closed the connection under the answer, and the client
			// read an EOF where it was told why it was refused.
			if hijacked.Load() {
				_ = listener.Close()
			}
		}),
		ConnState: func(_ net.Conn, state http.ConnState) {
			switch state {
			case http.StateHijacked:
				hijacked.Store(true)
			case http.StateClosed:
				// A connection closed before it asked for anything, or
				// after an answer that is no session.
				_ = listener.Close()
			}
		},
	}
	// One connection carries one request: the server closes it after an
	// answer that is no session, rather than waiting for another request.
	server.SetKeepAlivesEnabled(false)
	err := server.Serve(listener)
	_ = tlsConn.Close()
	if errors.Is(err, net.ErrClosed) || errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (cfg Config) handle(w http.ResponseWriter, r *http.Request, raw net.Conn) {
	refuse := func(code int, err error) {
		cfg.Log.Warnf("%s: refused: %s", r.RemoteAddr, err)
		w.Header().Set("Connection", "close")
		http.Error(w, err.Error(), code)
	}
	signed := r.URL.Query().Get("ticket")
	if signed == "" {
		refuse(http.StatusUnauthorized, errors.New("no console ticket"))
		return
	}
	ticket, err := console.ParseTicket(signed, cfg.VerifyKey)
	if err != nil {
		refuse(http.StatusUnauthorized, err)
		return
	}
	if err := useTicket(cfg.TicketDir, ticket.ID); err != nil {
		refuse(http.StatusUnauthorized, err)
		return
	}
	if cfg.Authenticated != nil {
		cfg.Authenticated()
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		cfg.Log.Warnf("%s: upgrade: %s", r.RemoteAddr, err)
		return
	}
	defer ws.Close()
	// The handshake is done: the session lasts as long as its client and
	// its command do.
	_ = raw.SetDeadline(time.Time{})

	log := cfg.Log.
		Attr("console_user", ticket.User()).
		Attr("console_path", ticket.Target.Path).
		Attr("console_rid", ticket.Target.RID).
		Attr("console_node", ticket.Target.Node).
		Attr("console_remote", r.RemoteAddr)
	what := fmt.Sprintf("%s %s on %s by %s from %s", ticket.Target.Path, ticket.Target.RID, ticket.Target.Node, ticket.User(), r.RemoteAddr)

	if ticket.Target.Node != cfg.Hostname {
		log.Infof("relay session: %s", what)
		err := cfg.relay(ws, ticket, signed)
		log.Infof("relay session ended: %s: %v", what, err)
		return
	}
	log.Infof("session: %s", what)
	code, err := cfg.run(ws, ticket, r.RemoteAddr)
	log.Infof("session ended: %s: exit code %d: %v", what, code, err)
}

// keepAlive pings the client until done is closed, and arms the deadline a
// client that stops answering is declared gone at.
func keepAlive(ws *websocket.Conn, w *wsWriter, done <-chan struct{}) {
	_ = ws.SetReadDeadline(time.Now().Add(pongTimeout))
	ws.SetPongHandler(func(string) error {
		return ws.SetReadDeadline(time.Now().Add(pongTimeout))
	})
	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				w.Lock()
				err := ws.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeTimeout))
				w.Unlock()
				if err != nil {
					return
				}
			}
		}
	}()
}

// run runs the command of the session in a terminal, connected to the
// client, until one of them ends. It returns the exit code of the command.
func (cfg Config) run(ws *websocket.Conn, ticket *console.Ticket, remote string) (int, error) {
	w := &wsWriter{conn: ws}
	fail := func(err error) (int, error) {
		w.end(console.Message{Code: 1, Reason: console.ReasonError, Text: err.Error()})
		return 1, err
	}
	if ticket.Target.Kind != console.KindTTY {
		return fail(fmt.Errorf("unsupported console kind: %s", ticket.Target.Kind))
	}
	args, err := cfg.Command(ticket.Target)
	if err != nil {
		return fail(err)
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	master, err := startInPTY(cmd)
	if err != nil {
		return fail(fmt.Errorf("start console command: %w", err))
	}
	defer master.Close()

	if unregister, err := register(cfg.SessionDir, Session{
		PID:       os.Getpid(),
		User:      ticket.User(),
		Remote:    remote,
		Target:    ticket.Target,
		StartedAt: time.Now(),
	}); err != nil {
		cfg.Log.Warnf("record the session: %s", err)
	} else {
		defer unregister()
	}

	done := make(chan struct{})
	defer close(done)
	keepAlive(ws, w, done)

	// The output of the terminal goes to the client.
	output := make(chan struct{})
	go func() {
		defer close(output)
		buf := make([]byte, 32*1024)
		for {
			n, err := master.Read(buf)
			if n > 0 {
				if err := w.write(websocket.BinaryMessage, buf[:n]); err != nil {
					return
				}
			}
			if err != nil {
				// EIO is how the master says the last process of
				// the terminal is gone.
				return
			}
		}
	}()

	// The keystrokes and the terminal sizes of the client go to the
	// terminal.
	clientGone := make(chan error, 1)
	go func() {
		for {
			messageType, b, err := ws.ReadMessage()
			if err != nil {
				clientGone <- err
				return
			}
			_ = ws.SetReadDeadline(time.Now().Add(pongTimeout))
			switch messageType {
			case websocket.BinaryMessage:
				if _, err := master.Write(b); err != nil {
					clientGone <- err
					return
				}
			case websocket.TextMessage:
				if m, err := console.DecodeMessage(b); err == nil && m.Type == console.MsgResize && m.Cols > 0 && m.Rows > 0 {
					_ = setPTYSize(master, m.Cols, m.Rows)
				}
			}
		}
	}()

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	select {
	case err := <-exited:
		// What the command wrote last is sent before the session is
		// declared over.
		select {
		case <-output:
		case <-time.After(time.Second):
		}
		code := exitCode(err)
		w.end(console.Message{Code: code, Reason: console.ReasonExited})
		return code, nil
	case err := <-clientGone:
		// A session ends with its client connection: the terminal is
		// hung up, which ends the command and what it started, and what
		// ignores the hang up is killed.
		_ = master.Close()
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGHUP)
		select {
		case werr := <-exited:
			return exitCode(werr), fmt.Errorf("client gone: %w", err)
		case <-time.After(killGrace):
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			werr := <-exited
			return exitCode(werr), fmt.Errorf("client gone, command killed: %w", err)
		}
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if code := exitErr.ExitCode(); code >= 0 {
			return code
		}
		// Ended by a signal.
		return 1
	}
	return 1
}

// relay connects the client to the session served by the node the ticket
// names, and copies the messages both ways until one side ends.
//
// A client reaches the console through whatever node it can reach, the one
// behind a virtual address or a site access proxy among them, and the
// session is served where the resource runs.
func (cfg Config) relay(ws *websocket.Conn, ticket *console.Ticket, signed string) error {
	w := &wsWriter{conn: ws}
	fail := func(err error) error {
		w.end(console.Message{Code: 1, Reason: console.ReasonError, Text: err.Error()})
		return err
	}
	if cfg.PeerURL == nil {
		return fail(fmt.Errorf("no route to the console of node %s", ticket.Target.Node))
	}
	peerURL, err := cfg.PeerURL(ticket.Target.Node)
	if err != nil {
		return fail(err)
	}
	u, err := url.Parse(peerURL)
	if err != nil {
		return fail(err)
	}
	q := u.Query()
	q.Set("ticket", signed)
	u.RawQuery = q.Encode()
	dialer := websocket.Dialer{
		TLSClientConfig:  cfg.PeerTLSConfig,
		HandshakeTimeout: handshakeTimeout,
	}
	peer, resp, err := dialer.Dial(u.String(), nil)
	if err != nil {
		if resp != nil {
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			_ = resp.Body.Close()
			return fail(fmt.Errorf("console of node %s: %s: %s", ticket.Target.Node, resp.Status, string(b)))
		}
		return fail(fmt.Errorf("console of node %s: %w", ticket.Target.Node, err))
	}
	defer peer.Close()
	pw := &wsWriter{conn: peer}

	done := make(chan struct{})
	defer close(done)
	keepAlive(ws, w, done)

	errs := make(chan error, 2)
	splice := func(from *websocket.Conn, to *wsWriter, client bool) {
		for {
			messageType, b, err := from.ReadMessage()
			if err != nil {
				// The end of one side is the end of the other, said
				// the way it was said.
				code, text := websocket.CloseGoingAway, ""
				var closeErr *websocket.CloseError
				if errors.As(err, &closeErr) && closeErr.Code != websocket.CloseNoStatusReceived && closeErr.Code != websocket.CloseAbnormalClosure {
					code, text = closeErr.Code, closeErr.Text
				}
				to.Lock()
				_ = to.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, text), time.Now().Add(writeTimeout))
				to.Unlock()
				errs <- err
				return
			}
			if client {
				_ = from.SetReadDeadline(time.Now().Add(pongTimeout))
			}
			if err := to.write(messageType, b); err != nil {
				errs <- err
				return
			}
		}
	}
	go splice(ws, pw, true)
	go splice(peer, w, false)
	err = <-errs
	if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
		return nil
	}
	return err
}
