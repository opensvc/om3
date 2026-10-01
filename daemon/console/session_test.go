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
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/opensvc/om3/v3/core/console"
	"github.com/opensvc/om3/v3/util/plog"
)

// testNode is a node serving console sessions in the process of the test.
type testNode struct {
	t    *testing.T
	name string
	url  string
	key  *rsa.PrivateKey
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
				_ = Serve(conn, n.cfg)
			})
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		n.wg.Wait()
	})
	return n
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
