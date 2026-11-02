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
	"io/fs"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/websocket"

	"github.com/opensvc/om3/v3/core/console"
	"github.com/opensvc/om3/v3/core/vpath"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/volfile"
)

type (
	// Config is what a session process needs of the node it runs on.
	Config struct {
		// Hostname is the name of this node: a ticket for another node
		// is relayed to it.
		Hostname string

		// Certificate is what the session authenticates the node to the
		// client with, unless GetCertificate returns one for the name
		// the client asks, as the api listener presents the
		// certificates of listener.tls_secs.
		Certificate    tls.Certificate
		GetCertificate func(*tls.ClientHelloInfo) (*tls.Certificate, error)

		// VerifyKey is the public key of the key the tickets are signed
		// with.
		VerifyKey *rsa.PublicKey

		// TicketDir is where the used tickets are marked, and SessionDir
		// where the sessions are recorded.
		TicketDir  string
		SessionDir string

		// Command returns the command a session runs in its terminal.
		Command func(console.Target) ([]string, error)

		// Graphics returns a connection to the display of a target.
		Graphics func(console.Target) (net.Conn, error)

		// Upload writes r to the file of a target, for the user named.
		Upload func(target console.Target, user string, r io.Reader) error

		// Download opens the file of a target, for reading.
		Download func(target console.Target) (*os.File, error)

		// PeerURL returns the url of the console endpoint of a peer
		// node, and PeerTLSConfig how to verify it. A PeerTLSConfig
		// naming no server verifies the certificate of the peer for its
		// node name, the url possibly being of its address.
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

	// uploadIdle is how long a transfer waits for its client to send
	// something: this process lasts as long as the transfer does.
	uploadIdle = time.Minute

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
		Certificates:   []tls.Certificate{cfg.Certificate},
		GetCertificate: cfg.GetCertificate,
		// The minimum of the api listener: the clients reaching one
		// reach the other.
		MinVersion: tls.VersionTLS13,
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
	// The ticket is what authorizes a transfer, and a page of another origin
	// does not have it, as for a websocket: see the upgrader.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", http.MethodPut+", "+http.MethodGet)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Range")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Content-Disposition, Accept-Ranges")
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
	switch ticket.Target.Kind {
	case console.KindUpload:
		cfg.handleUpload(w, r, raw, ticket, signed)
		return
	case console.KindDownload:
		cfg.handleDownload(w, r, raw, ticket, signed)
		return
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
	if ticket.Target.Kind == console.KindGraphics {
		return cfg.runGraphics(ws, ticket, remote)
	}
	w := &wsWriter{conn: ws}
	fail := func(err error) (int, error) {
		w.end(console.Message{Code: 1, Reason: console.ReasonError, Text: err.Error()})
		return 1, err
	}
	if ticket.Target.Kind != console.KindTTY && ticket.Target.Kind != console.KindShell {
		return fail(fmt.Errorf("unsupported console kind: %s", ticket.Target.Kind))
	}
	args, err := cfg.Command(ticket.Target)
	if err != nil {
		return fail(err)
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	if ticket.Target.Kind == console.KindShell {
		// A login shell of the user, not one of the environment of the
		// daemon, which systemd gives no home.
		if u, err := user.Current(); err == nil {
			cmd.Dir = u.HomeDir
			cmd.Env = append(cmd.Env, "HOME="+u.HomeDir, "USER="+u.Username, "LOGNAME="+u.Username)
		}
	}
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

// idleReader reads a client, ending the connection of one that stops sending.
type idleReader struct {
	r    io.Reader
	conn net.Conn
}

func (t idleReader) Read(p []byte) (int, error) {
	_ = t.conn.SetReadDeadline(time.Now().Add(uploadIdle))
	return t.r.Read(p)
}

// handleUpload receives the file of a ticket, or relays it to the node of the
// ticket. Nothing but the bytes goes through: the answer says how it ended.
func (cfg Config) handleUpload(w http.ResponseWriter, r *http.Request, raw net.Conn, ticket *console.Ticket, signed string) {
	if r.Method != http.MethodPut {
		http.Error(w, "a file is sent by a PUT", http.StatusMethodNotAllowed)
		return
	}
	// The handshake is done: the transfer lasts as long as its client sends.
	_ = raw.SetDeadline(time.Time{})
	body := idleReader{r: r.Body, conn: raw}
	target := ticket.Target
	what := fmt.Sprintf("%s %s on %s by %s from %s", target.Path, target.File, target.Node, ticket.User(), r.RemoteAddr)
	if target.Node != cfg.Hostname {
		cfg.Log.Infof("relay upload: %s", what)
		if err := cfg.relayUpload(w, body, r.ContentLength, ticket, signed); err != nil {
			cfg.Log.Warnf("relay upload ended: %s: %s", what, err)
			http.Error(w, err.Error(), http.StatusBadGateway)
		}
		return
	}
	if cfg.Upload == nil {
		http.Error(w, "no upload on this node", http.StatusInternalServerError)
		return
	}
	if unregister, err := register(cfg.SessionDir, Session{
		PID:       os.Getpid(),
		User:      ticket.User(),
		Remote:    r.RemoteAddr,
		Target:    target,
		StartedAt: time.Now(),
	}); err != nil {
		cfg.Log.Warnf("record the session: %s", err)
	} else {
		defer unregister()
	}
	cfg.Log.Infof("upload: %s", what)
	err := cfg.Upload(target, ticket.User(), body)
	cfg.Log.Infof("upload ended: %s: %v", what, err)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, volfile.ErrExists):
		http.Error(w, fmt.Sprintf("file exists: %s, not replaced", target.File), http.StatusConflict)
	case errors.Is(err, volfile.ErrInstalled):
		http.Error(w, fmt.Sprintf("installed file: %s, not replaced", target.File), http.StatusConflict)
	case errors.Is(err, volfile.ErrNoDir):
		http.Error(w, fmt.Sprintf("no such directory: %s", target.Dir), http.StatusNotFound)
	case errors.Is(err, volfile.ErrNotDir):
		http.Error(w, fmt.Sprintf("not a directory: %s", target.Dir), http.StatusConflict)
	case errors.Is(err, volfile.ErrInvalidDir):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, volfile.ErrFull), errors.Is(err, syscall.ENOSPC), errors.Is(err, syscall.EDQUOT):
		http.Error(w, fmt.Sprintf("%s is full: grow the volume", target.Path), http.StatusInsufficientStorage)
	case errors.As(err, &vpath.ErrAccess{}):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// idleWriter cuts a transfer whose client reads nothing for uploadIdle.
type idleWriter struct {
	http.ResponseWriter
	conn net.Conn
}

func (t idleWriter) Write(p []byte) (int, error) {
	_ = t.conn.SetWriteDeadline(time.Now().Add(uploadIdle))
	return t.ResponseWriter.Write(p)
}

// handleDownload sends the file of a ticket, or relays the read to the node of
// the ticket. A Range is honoured, for a client to read the start of a
// file, as a view does, rather than all of it.
func (cfg Config) handleDownload(w http.ResponseWriter, r *http.Request, raw net.Conn, ticket *console.Ticket, signed string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "a file is read by a GET", http.StatusMethodNotAllowed)
		return
	}
	// The handshake is done: the transfer lasts as long as its client reads.
	_ = raw.SetDeadline(time.Time{})
	target := ticket.Target
	what := fmt.Sprintf("%s %s on %s by %s from %s", target.Path, path.Join(target.Dir, target.File), target.Node, ticket.User(), r.RemoteAddr)
	if target.Node != cfg.Hostname {
		cfg.Log.Infof("relay download: %s", what)
		if err := cfg.relayDownload(idleWriter{ResponseWriter: w, conn: raw}, r, ticket, signed); err != nil {
			cfg.Log.Warnf("relay download ended: %s: %s", what, err)
			http.Error(w, err.Error(), http.StatusBadGateway)
		}
		return
	}
	if cfg.Download == nil {
		http.Error(w, "no download on this node", http.StatusInternalServerError)
		return
	}
	f, err := cfg.Download(target)
	switch {
	case err == nil:
	case errors.Is(err, fs.ErrNotExist):
		http.Error(w, fmt.Sprintf("no such file: %s", target.File), http.StatusNotFound)
		return
	case errors.Is(err, volfile.ErrNotFile):
		http.Error(w, fmt.Sprintf("not a file: %s", target.File), http.StatusConflict)
		return
	case errors.Is(err, volfile.ErrNoDir):
		http.Error(w, fmt.Sprintf("no such directory: %s", target.Dir), http.StatusNotFound)
		return
	case errors.Is(err, volfile.ErrNotDir):
		http.Error(w, fmt.Sprintf("not a directory: %s", target.Dir), http.StatusConflict)
		return
	case errors.Is(err, volfile.ErrInvalidDir):
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	case errors.As(err, &vpath.ErrAccess{}):
		http.Error(w, err.Error(), http.StatusConflict)
		return
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if unregister, err := register(cfg.SessionDir, Session{
		PID:       os.Getpid(),
		User:      ticket.User(),
		Remote:    r.RemoteAddr,
		Target:    target,
		StartedAt: time.Now(),
	}); err != nil {
		cfg.Log.Warnf("record the session: %s", err)
	} else {
		defer unregister()
	}
	cfg.Log.Infof("download: %s", what)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": target.File}))
	http.ServeContent(idleWriter{ResponseWriter: w, conn: raw}, r, "", info.ModTime(), f)
	cfg.Log.Infof("download ended: %s", what)
}

// relayDownload reads a file from the console of the node of the ticket, and
// answers what that node answers.
func (cfg Config) relayDownload(w http.ResponseWriter, r *http.Request, ticket *console.Ticket, signed string) error {
	u, client, err := cfg.peerConsole(ticket, signed)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(r.Context(), r.Method, u, nil)
	if err != nil {
		return err
	}
	for _, h := range []string{"Range", "If-Range"} {
		if v := r.Header.Get(h); v != "" {
			req.Header.Set(h, v)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("console of node %s: %w", ticket.Target.Node, withoutURL(err))
	}
	defer func() { _ = resp.Body.Close() }()
	for _, h := range []string{"Content-Type", "Content-Length", "Content-Range", "Content-Disposition", "Accept-Ranges", "Last-Modified"} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
	return nil
}

// withoutURL returns the error of a request without its url, which holds the
// signed ticket: the error is logged, and answered.
func withoutURL(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return fmt.Errorf("%s: %w", urlErr.Op, urlErr.Err)
	}
	return err
}

// peerConsole returns the url of the console of the node of the ticket, with
// the ticket, and the client to reach it.
func (cfg Config) peerConsole(ticket *console.Ticket, signed string) (string, *http.Client, error) {
	if cfg.PeerURL == nil {
		return "", nil, fmt.Errorf("no route to the console of node %s", ticket.Target.Node)
	}
	peerURL, err := cfg.PeerURL(ticket.Target.Node)
	if err != nil {
		return "", nil, err
	}
	u, err := url.Parse(peerURL)
	if err != nil {
		return "", nil, err
	}
	u.Scheme = "https"
	q := u.Query()
	q.Set("ticket", signed)
	u.RawQuery = q.Encode()
	tlsConfig := cfg.PeerTLSConfig.Clone()
	if tlsConfig != nil && tlsConfig.ServerName == "" {
		tlsConfig.ServerName = ticket.Target.Node
	}
	return u.String(), &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig}}, nil
}

// relayUpload sends a transfer on to the console of the node of the ticket, and
// answers what that node answers.
func (cfg Config) relayUpload(w http.ResponseWriter, body io.Reader, size int64, ticket *console.Ticket, signed string) error {
	u, client, err := cfg.peerConsole(ticket, signed)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, u, body)
	if err != nil {
		return err
	}
	req.ContentLength = size
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("console of node %s: %w", ticket.Target.Node, withoutURL(err))
	}
	defer func() { _ = resp.Body.Close() }()
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(resp.Body, 4096))
	return nil
}

// runGraphics relays the bytes of a display and of its client. Nothing but
// them goes through the session: the client of the display protocol takes no
// control message, so an end is said by the close message of the websocket.
func (cfg Config) runGraphics(ws *websocket.Conn, ticket *console.Ticket, remote string) (int, error) {
	closeWith := func(code int, reason string) {
		// The reason of a control message is limited to 123 bytes.
		_ = ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason[:min(len(reason), 123)]), time.Now().Add(writeTimeout))
	}
	if cfg.Graphics == nil {
		closeWith(websocket.CloseInternalServerErr, "no display on this node")
		return 1, errors.New("no display on this node")
	}
	display, err := cfg.Graphics(ticket.Target)
	if err != nil {
		closeWith(websocket.CloseInternalServerErr, err.Error())
		return 1, err
	}
	defer display.Close()

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

	w := &wsWriter{conn: ws}
	done := make(chan struct{})
	defer close(done)
	keepAlive(ws, w, done)

	ended := make(chan error, 2)
	go func() {
		buf := make([]byte, 64*1024)
		for {
			n, err := display.Read(buf)
			if n > 0 {
				if err := w.write(websocket.BinaryMessage, buf[:n]); err != nil {
					ended <- err
					return
				}
			}
			if err != nil {
				ended <- fmt.Errorf("display: %w", err)
				return
			}
		}
	}()
	go func() {
		for {
			messageType, b, err := ws.ReadMessage()
			if err != nil {
				ended <- fmt.Errorf("client gone: %w", err)
				return
			}
			_ = ws.SetReadDeadline(time.Now().Add(pongTimeout))
			if messageType != websocket.BinaryMessage {
				continue
			}
			if _, err := display.Write(b); err != nil {
				ended <- fmt.Errorf("display: %w", err)
				return
			}
		}
	}()
	err = <-ended
	closeWith(websocket.CloseNormalClosure, "")
	return 0, err
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
	tlsConfig := cfg.PeerTLSConfig.Clone()
	if tlsConfig != nil && tlsConfig.ServerName == "" {
		tlsConfig.ServerName = ticket.Target.Node
	}
	dialer := websocket.Dialer{
		TLSClientConfig:  tlsConfig,
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
