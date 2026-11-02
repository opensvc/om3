//go:build linux

package console

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gorilla/websocket"

	"github.com/opensvc/om3/v3/core/console"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/vpath"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/volfile"
)

// testNode is a node serving console sessions in the process of the test.
type testNode struct {
	t    *testing.T
	name string
	url  string
	key  *rsa.PrivateKey
	mu   sync.Mutex
	cfg  Config
	wg   sync.WaitGroup
}

var clientTLS = &tls.Config{InsecureSkipVerify: true}

func testCertificate(t *testing.T) (tls.Certificate, []byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "console test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return cert, certPEM, keyPEM
}

// startTestNode serves the sessions of a node named name, each running the
// shell script given, with the key the tickets are signed with.
func startTestNode(t *testing.T, name string, key *rsa.PrivateKey, script string) *testNode {
	t.Helper()
	return startTestNodeWithPeers(t, name, key, script, nil)
}

// startTestNodeWithPeers is startTestNode, with the route to the peers the
// node relays to.
func startTestNodeWithPeers(t *testing.T, name string, key *rsa.PrivateKey, script string, peerURL func(string) (string, error)) *testNode {
	t.Helper()
	cert, _, _ := testCertificate(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	n := &testNode{
		t:    t,
		name: name,
		url:  "wss://" + listener.Addr().String() + "/",
		key:  key,
	}
	n.cfg = Config{
		Hostname:    name,
		Certificate: cert,
		VerifyKey:   &key.PublicKey,
		TicketDir:   filepath.Join(dir, "tickets"),
		SessionDir:  filepath.Join(dir, "sessions"),
		Command: func(console.Target) ([]string, error) {
			return []string{"/bin/sh", "-c", script}, nil
		},
		PeerURL:       peerURL,
		PeerTLSConfig: clientTLS,
		Log:           plog.NewDefaultLogger().WithPrefix("test console " + name + ": "),
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			n.wg.Go(func() {
				defer conn.Close()
				_ = Serve(conn, n.config())
			})
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		n.wg.Wait()
	})
	return n
}

// config returns what a connection is served with: a test sets it while the
// listener runs.
func (n *testNode) config() Config {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.cfg
}

func (n *testNode) setConfig(set func(*Config)) {
	n.mu.Lock()
	defer n.mu.Unlock()
	set(&n.cfg)
}

// ticket returns a signed ticket for a session on the node named.
func (n *testNode) ticket(node string) string {
	n.t.Helper()
	ticket, err := console.NewTicket("alice", n.name, console.Target{Node: node, Path: "ns1/svc/web", RID: "container#1", Kind: console.KindTTY})
	if err != nil {
		n.t.Fatal(err)
	}
	signed, err := ticket.Sign(n.key)
	if err != nil {
		n.t.Fatal(err)
	}
	return signed
}

// attach opens a session on the node with the ticket, feeding it the input,
// and returns what it wrote and how it ended.
func (n *testNode) attach(ctx context.Context, ticket, input string) (string, console.Result, error) {
	n.t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		n.t.Fatal(err)
	}
	defer r.Close()
	go func() {
		_, _ = w.WriteString(input)
		// The pipe stays open: a session does not end with the input of
		// its client, it ends with its command.
	}()
	defer w.Close()
	var out bytes.Buffer
	result, err := console.Attach(ctx, console.AttachOptions{
		URL:       n.url + "?ticket=" + ticket,
		TLSConfig: clientTLS,
		Stdin:     r,
		Stdout:    &out,
	})
	return out.String(), result, err
}

func testRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// A session runs its command in a terminal: what the command writes reaches
// the client, what the client types reaches the command, and the session
// ends with the exit code of the command.
func TestSession(t *testing.T) {
	n := startTestNode(t, "n1", testRSAKey(t), `echo hello; read x; echo "got:$x"; test -t 0 && echo tty; exit 3`)
	out, result, err := n.attach(context.Background(), n.ticket("n1"), "abc\n")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"hello", "got:abc", "tty"} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q does not hold %q", out, want)
		}
	}
	if result.Code != 3 || result.Reason != console.ReasonExited {
		t.Errorf("result %+v", result)
	}
}

// A session is recorded for the time it lasts, with who opened it and on
// what.
func TestSessionIsRecorded(t *testing.T) {
	n := startTestNode(t, "n1", testRSAKey(t), `echo ready; read x`)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	var out syncBuffer
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = console.Attach(context.Background(), console.AttachOptions{
			URL: n.url + "?ticket=" + n.ticket("n1"), TLSConfig: clientTLS, Stdin: r, Stdout: &out,
		})
	}()
	waitFor(t, "the session to start", func() bool { return strings.Contains(out.String(), "ready") })
	entries, err := os.ReadDir(n.cfg.SessionDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("session records: %v, %v", entries, err)
	}
	b, err := os.ReadFile(filepath.Join(n.cfg.SessionDir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"user":"alice"`, `"path":"ns1/svc/web"`, `"rid":"container#1"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("record %s does not hold %s", b, want)
		}
	}
	_, _ = w.WriteString("\n")
	<-done
	waitFor(t, "the record to be removed", func() bool {
		entries, _ := os.ReadDir(n.cfg.SessionDir)
		return len(entries) == 0
	})
}

// A ticket opens one session: presented again, it is refused, and so is a
// connection with no ticket, or with a ticket another key signed.
func TestSessionRefused(t *testing.T) {
	n := startTestNode(t, "n1", testRSAKey(t), `echo hello`)
	ticket := n.ticket("n1")
	if _, _, err := n.attach(context.Background(), ticket, ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := n.attach(context.Background(), ticket, ""); err == nil || !strings.Contains(err.Error(), "already used") {
		t.Errorf("a ticket used twice: %v", err)
	}
	if _, _, err := n.attach(context.Background(), "", ""); err == nil {
		t.Error("a session with no ticket is opened")
	}
	other := &testNode{t: t, name: "n1", key: testRSAKey(t)}
	if _, _, err := n.attach(context.Background(), other.ticket("n1"), ""); err == nil {
		t.Error("a ticket signed by another key is accepted")
	}
}

// The size of the client terminal is the size of the session terminal.
func TestSessionResize(t *testing.T) {
	n := startTestNode(t, "n1", testRSAKey(t), `read x; stty size`)
	dialer := websocket.Dialer{TLSClientConfig: clientTLS}
	ws, _, err := dialer.Dial(n.url+"?ticket="+n.ticket("n1"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	if err := ws.WriteMessage(websocket.TextMessage, console.Message{Type: console.MsgResize, Cols: 132, Rows: 43}.Encode()); err != nil {
		t.Fatal(err)
	}
	if err := ws.WriteMessage(websocket.BinaryMessage, []byte("\n")); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	for {
		messageType, b, err := ws.ReadMessage()
		if err != nil {
			break
		}
		if messageType == websocket.BinaryMessage {
			out.Write(b)
		}
	}
	if !strings.Contains(out.String(), "43 132") {
		t.Errorf("terminal size not applied: %q", out.String())
	}
}

// A session for another node is relayed to it: the client reaches the
// console through the node it can reach.
func TestSessionRelay(t *testing.T) {
	key := testRSAKey(t)
	served := startTestNode(t, "n2", key, `echo "on n2"; read x; echo "got:$x"; exit 4`)
	entry := startTestNodeWithPeers(t, "n1", key, `echo "on n1"`, func(node string) (string, error) {
		if node != "n2" {
			return "", fmt.Errorf("unknown node %s", node)
		}
		return served.url, nil
	})
	out, result, err := entry.attach(context.Background(), entry.ticket("n2"), "xyz\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "on n2") || !strings.Contains(out, "got:xyz") || strings.Contains(out, "on n1") {
		t.Errorf("output %q", out)
	}
	if result.Code != 4 || result.Reason != console.ReasonExited {
		t.Errorf("result %+v", result)
	}

	// A node that can not be reached is an error the client is told.
	_, result, err = entry.attach(context.Background(), entry.ticket("n3"), "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Reason != console.ReasonError || !strings.Contains(result.Text, "n3") {
		t.Errorf("relay to an unknown node: %+v", result)
	}
}

// A session ends with its client connection: the command and what it
// started are ended too.
func TestSessionEndsWithItsClient(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")
	n := startTestNode(t, "n1", testRSAKey(t), `echo $$ > `+pidFile+`; echo ready; sleep 300`)
	ctx, cancel := context.WithCancel(context.Background())
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	var out syncBuffer
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = console.Attach(ctx, console.AttachOptions{
			URL: n.url + "?ticket=" + n.ticket("n1"), TLSConfig: clientTLS, Stdin: r, Stdout: &out,
		})
	}()
	waitFor(t, "the session to start", func() bool { return strings.Contains(out.String(), "ready") })
	b, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Kill(pid, 0); err != nil {
		t.Fatalf("the command is not running: %s", err)
	}
	cancel()
	<-done
	waitFor(t, "the command to end", func() bool { return syscall.Kill(pid, 0) != nil })
}

// syncBuffer is a buffer written by the session and read by the test.
type syncBuffer struct {
	sync.Mutex
	b bytes.Buffer
}

func (t *syncBuffer) Write(p []byte) (int, error) {
	t.Lock()
	defer t.Unlock()
	return t.b.Write(p)
}

func (t *syncBuffer) String() string {
	t.Lock()
	defer t.Unlock()
	return t.b.String()
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
}

// handshake opens a tls connection to a session served with the config, and
// returns the certificate it is presented.
func handshake(t *testing.T, cfg Config, clientConfig *tls.Config) (*x509.Certificate, error) {
	t.Helper()
	serverConn, clientConn := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = Serve(serverConn, cfg)
	}()
	defer func() {
		_ = clientConn.Close()
		<-done
	}()
	conn := tls.Client(clientConn, clientConfig)
	if err := conn.Handshake(); err != nil {
		return nil, err
	}
	return conn.ConnectionState().PeerCertificates[0], nil
}

// The console presents the certificate of the name the client asks, as the
// api listener presents the ones of listener.tls_secs, and its own to the
// others.
func TestSessionCertificateByName(t *testing.T) {
	cert, _, _ := testCertificate(t)
	named, _, _ := testCertificate(t)
	cfg := Config{
		Certificate: cert,
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			if hello.ServerName == "console.acme.com" {
				return &named, nil
			}
			return nil, nil
		},
		Log: plog.NewDefaultLogger().WithPrefix("test console: "),
	}
	for name, want := range map[string]tls.Certificate{"console.acme.com": named, "n1": cert} {
		got, err := handshake(t, cfg, &tls.Config{InsecureSkipVerify: true, ServerName: name})
		if err != nil {
			t.Fatalf("%s: %s", name, err)
		}
		if !bytes.Equal(got.Raw, want.Certificate[0]) {
			t.Errorf("%s: not presented the certificate of the name", name)
		}
	}
}

// The console requires tls 1.3, as the api listener does.
func TestSessionRefusesTLS12(t *testing.T) {
	cert, _, _ := testCertificate(t)
	cfg := Config{Certificate: cert, Log: plog.NewDefaultLogger().WithPrefix("test console: ")}
	if _, err := handshake(t, cfg, &tls.Config{InsecureSkipVerify: true, MaxVersion: tls.VersionTLS12}); err == nil {
		t.Fatal("a tls 1.2 client is served")
	}
}

func TestShellOf(t *testing.T) {
	const passwd = "root:x:0:0:root:/root:/bin/bash\nnoshell:x:1:1::/:\nshort:x\nzsh:x:2:2::/home/zsh:/bin/zsh\n"
	for name, want := range map[string]string{
		"root":    "/bin/bash",
		"zsh":     "/bin/zsh",
		"noshell": "/bin/sh",
		"short":   "/bin/sh",
		"missing": "/bin/sh",
	} {
		if got := shellOf(passwd, name, "/bin/sh"); got != want {
			t.Errorf("%s: got %s, want %s", name, got, want)
		}
	}
}

func TestEnterCommandOfAShell(t *testing.T) {
	args, err := enterCommand(console.Target{Node: "vm1", Kind: console.KindShell})
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 2 || args[1] != "-l" {
		t.Errorf("got %v", args)
	}
}

// graphicsSession opens a session of the display the dial function connects to.
func graphicsSession(t *testing.T, dial func(console.Target) (net.Conn, error)) *websocket.Conn {
	t.Helper()
	key := testRSAKey(t)
	n := startTestNode(t, "node1", key, "true")
	n.setConfig(func(cfg *Config) { cfg.Graphics = dial })
	ticket, err := console.NewTicket("alice", "node1", console.Target{Node: "node1", Path: "ns1/svc/web", RID: "container#1", Kind: console.KindGraphics})
	require.NoError(t, err)
	signed, err := ticket.Sign(key)
	require.NoError(t, err)
	dialer := websocket.Dialer{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	ws, _, err := dialer.Dial(n.url+"?ticket="+url.QueryEscape(signed), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ws.Close() })
	return ws
}

// The bytes of a display go through the session both ways, untouched.
func TestSessionGraphics(t *testing.T) {
	display, session := net.Pipe()
	defer display.Close()
	ws := graphicsSession(t, func(console.Target) (net.Conn, error) { return session, nil })

	_, err := display.Write([]byte("RFB 003.008\n"))
	require.NoError(t, err)
	messageType, b, err := ws.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, websocket.BinaryMessage, messageType)
	assert.Equal(t, "RFB 003.008\n", string(b))

	require.NoError(t, ws.WriteMessage(websocket.BinaryMessage, []byte("RFB 003.008\n")))
	buf := make([]byte, 32)
	n, err := display.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "RFB 003.008\n", string(buf[:n]))

	// The display ending ends the session.
	_ = display.Close()
	_, _, err = ws.ReadMessage()
	assert.True(t, websocket.IsCloseError(err, websocket.CloseNormalClosure), "%v", err)
}

// A client of the display protocol takes no control message: a failure is the
// reason of the close message.
func TestSessionGraphicsRefused(t *testing.T) {
	ws := graphicsSession(t, func(console.Target) (net.Conn, error) {
		return nil, errors.New("the guest has a spice display, and a vnc one is needed")
	})
	_, _, err := ws.ReadMessage()
	var closeErr *websocket.CloseError
	require.ErrorAs(t, err, &closeErr)
	assert.Equal(t, "the guest has a spice display, and a vnc one is needed", closeErr.Text)
}

// uploadNode starts a node storing what it is sent in a buffer.
func uploadNode(t *testing.T, name string, key *rsa.PrivateKey, peerURL func(string) (string, error), upload func(console.Target, string, io.Reader) error) *testNode {
	t.Helper()
	n := startTestNodeWithPeers(t, name, key, "true", peerURL)
	n.setConfig(func(cfg *Config) { cfg.Upload = upload })
	return n
}

func uploadTicket(t *testing.T, key *rsa.PrivateKey, node string) string {
	t.Helper()
	ticket, err := console.NewTicket("alice", "node1", console.Target{Node: node, Path: "ns1/vol/images", Kind: console.KindUpload, File: "a.img"})
	require.NoError(t, err)
	signed, err := ticket.Sign(key)
	require.NoError(t, err)
	return signed
}

func put(t *testing.T, n *testNode, ticket, body string) (int, string) {
	t.Helper()
	u := strings.Replace(n.url, "wss://", "https://", 1) + "?ticket=" + url.QueryEscape(ticket)
	req, err := http.NewRequest(http.MethodPut, u, strings.NewReader(body))
	require.NoError(t, err)
	c := http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	resp, err := c.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"), "a page of the api reads the answer")
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestSessionUpload(t *testing.T) {
	key := testRSAKey(t)
	var got syncBuffer
	var (
		target console.Target
		user   string
	)
	n := uploadNode(t, "node1", key, nil, func(tgt console.Target, u string, r io.Reader) error {
		target, user = tgt, u
		_, err := io.Copy(&got, r)
		return err
	})
	ticket := uploadTicket(t, key, "node1")
	code, _ := put(t, n, ticket, "the bytes")
	assert.Equal(t, http.StatusNoContent, code)
	assert.Equal(t, "the bytes", got.String())
	assert.Equal(t, "a.img", target.File)
	assert.Equal(t, "alice", user, "the upload is marked as the user of the ticket")

	code, body := put(t, n, ticket, "again")
	assert.Equal(t, http.StatusUnauthorized, code, "a ticket opens one transfer")
	assert.Contains(t, body, "already used")
}

func TestSessionUploadRefused(t *testing.T) {
	key := testRSAKey(t)
	for name, tc := range map[string]struct {
		err  error
		code int
	}{
		"full":          {volfile.ErrFull, http.StatusInsufficientStorage},
		"file exists":   {fmt.Errorf("%w: a.img", volfile.ErrExists), http.StatusConflict},
		"no directory":  {fmt.Errorf("%w: isos", volfile.ErrNoDir), http.StatusNotFound},
		"not directory": {fmt.Errorf("%w: isos", volfile.ErrNotDir), http.StatusConflict},
		"no disk space": {fmt.Errorf("write: %w", syscall.ENOSPC), http.StatusInsufficientStorage},
		"not available": {vpath.ErrAccess{Avail: status.Down}, http.StatusConflict},
		"anything else": {errors.New("boom"), http.StatusInternalServerError},
	} {
		n := uploadNode(t, "node1", key, nil, func(console.Target, string, io.Reader) error { return tc.err })
		code, _ := put(t, n, uploadTicket(t, key, "node1"), "x")
		assert.Equal(t, tc.code, code, name)
	}
}

// A page of another origin asks before it sends, with no ticket to use up.
func TestSessionUploadPreflight(t *testing.T) {
	key := testRSAKey(t)
	n := uploadNode(t, "node1", key, nil, func(console.Target, string, io.Reader) error { return nil })
	u := strings.Replace(n.url, "wss://", "https://", 1)
	req, err := http.NewRequest(http.MethodOptions, u, nil)
	require.NoError(t, err)
	c := http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	resp, err := c.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, http.MethodPut, resp.Header.Get("Access-Control-Allow-Methods"))
}

// A transfer reaching another node than the one of the volume goes on to it,
// whose answer the client gets.
func TestSessionUploadRelay(t *testing.T) {
	key := testRSAKey(t)
	var got syncBuffer
	node2 := uploadNode(t, "node2", key, nil, func(_ console.Target, _ string, r io.Reader) error {
		_, err := io.Copy(&got, r)
		return err
	})
	node1 := uploadNode(t, "node1", key, func(string) (string, error) { return node2.url, nil }, nil)
	code, _ := put(t, node1, uploadTicket(t, key, "node2"), "relayed")
	assert.Equal(t, http.StatusNoContent, code)
	assert.Equal(t, "relayed", got.String())
}

// A record of a process killed goes with the next session, and one of a
// process alive stays.
func TestRegisterSweepsDeadSessions(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("true")
	require.NoError(t, cmd.Run())
	dead := filepath.Join(dir, fmt.Sprintf("%d.json", cmd.Process.Pid))
	alive := filepath.Join(dir, fmt.Sprintf("%d.json", os.Getppid()))
	require.NoError(t, os.WriteFile(dead, []byte("{}"), 0o600))
	require.NoError(t, os.WriteFile(alive, []byte("{}"), 0o600))
	_, err := register(dir, Session{PID: os.Getpid()})
	require.NoError(t, err)
	assert.NoFileExists(t, dead)
	assert.FileExists(t, alive)
}
