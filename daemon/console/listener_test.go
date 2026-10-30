//go:build linux

package console

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opensvc/om3/v3/core/console"
	"github.com/opensvc/om3/v3/util/plog"
)

// sessionDirVar names the directory a test binary run as a session process
// finds what it serves the session with.
const sessionDirVar = "OM3_CONSOLE_TEST_SESSION_DIR"

// TestMain runs the test binary as a session process when the listener under
// test started it as one: the listener hands a connection to a process, and
// the process here is the test binary itself.
func TestMain(m *testing.M) {
	if dir := os.Getenv(sessionDirVar); dir != "" {
		os.Exit(serveTestSession(dir))
	}
	os.Exit(m.Run())
}

func serveTestSession(dir string) int {
	cert, err := tls.LoadX509KeyPair(filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem"))
	if err != nil {
		return 2
	}
	b, err := os.ReadFile(filepath.Join(dir, "verify.pem"))
	if err != nil {
		return 2
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return 2
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return 2
	}
	connFile := os.NewFile(sessionConnFD, "conn")
	conn, err := net.FileConn(connFile)
	if err != nil {
		return 2
	}
	_ = connFile.Close()
	authFile := os.NewFile(sessionAuthFD, "auth")
	if err := Serve(conn, Config{
		Hostname:    "n1",
		Certificate: cert,
		VerifyKey:   pub.(*rsa.PublicKey),
		TicketDir:   filepath.Join(dir, "tickets"),
		SessionDir:  filepath.Join(dir, "sessions"),
		Command: func(console.Target) ([]string, error) {
			return []string{"/bin/sh", "-c", `echo ready; read x; echo "got:$x"; read y; echo "then:$y"`}, nil
		},
		Authenticated: func() { _ = authFile.Close() },
		Log:           plog.NewDefaultLogger().WithPrefix("test console session: "),
	}); err != nil {
		return 1
	}
	return 0
}

// startTestListener starts a listener handing its connections to the test
// binary run as a session process.
func startTestListener(t *testing.T) (*Listener, *testNode) {
	t.Helper()
	dir := t.TempDir()
	key := testRSAKey(t)
	_, certPEM, keyPEM := testCertificate(t)
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	for name, b := range map[string][]byte{
		"cert.pem":   certPEM,
		"key.pem":    keyPEM,
		"verify.pem": pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	l := NewListener("127.0.0.1:0")
	l.command = func() *exec.Cmd {
		cmd := exec.Command(os.Args[0])
		cmd.Env = append(os.Environ(), sessionDirVar+"="+dir)
		return cmd
	}
	if err := l.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	n := &testNode{
		t:    t,
		name: "n1",
		url:  "wss://" + l.listener.Addr().String() + "/",
		key:  key,
		cfg:  Config{SessionDir: filepath.Join(dir, "sessions")},
	}
	return l, n
}

// The listener hands a connection to a process of its own, and a session
// lasts after the listener stopped, as it does after the daemon holding the
// listener is restarted.
func TestListenerSessionOutlivesTheListener(t *testing.T) {
	l, n := startTestListener(t)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	var out syncBuffer
	type ended struct {
		result console.Result
		err    error
	}
	done := make(chan ended, 1)
	go func() {
		result, err := console.Attach(context.Background(), console.AttachOptions{
			URL: n.url + "?ticket=" + n.ticket("n1"), TLSConfig: clientTLS, Stdin: r, Stdout: &out,
		})
		done <- ended{result, err}
	}()
	waitFor(t, "the session to start", func() bool { return strings.Contains(out.String(), "ready") })
	waitFor(t, "the session to be counted as authenticated", func() bool { return l.unauthenticated.Load() == 0 })
	_, _ = w.WriteString("before\n")
	waitFor(t, "the first answer", func() bool { return strings.Contains(out.String(), "got:before") })

	if err := l.Stop(); err != nil {
		t.Fatal(err)
	}
	if _, err := net.Dial("tcp", strings.TrimSuffix(strings.TrimPrefix(n.url, "wss://"), "/")); err == nil {
		t.Error("the listener still accepts after it stopped")
	}

	_, _ = w.WriteString("after\n")
	e := <-done
	if e.err != nil {
		t.Fatal(e.err)
	}
	if !strings.Contains(out.String(), "then:after") {
		t.Errorf("the session did not go on after the listener stopped: %q", out.String())
	}
	if e.result.Reason != console.ReasonExited || e.result.Code != 0 {
		t.Errorf("result %+v", e.result)
	}
}

// The connections that present no ticket are bounded in number: past the
// bound, a connection is closed without a process being started for it, and
// the ones that gave up make room again.
func TestListenerBoundsTheUnauthenticated(t *testing.T) {
	l, n := startTestListener(t)
	defer l.Stop()
	addr := strings.TrimSuffix(strings.TrimPrefix(n.url, "wss://"), "/")
	var conns []net.Conn
	defer func() {
		for _, c := range conns {
			_ = c.Close()
		}
	}()
	for i := 0; i < maxUnauthenticated; i++ {
		c, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		conns = append(conns, c)
	}
	waitFor(t, "the silent connections to be counted", func() bool { return l.unauthenticated.Load() == maxUnauthenticated })

	// One more is closed at once: reading it answers the end of the
	// connection, where a session process would wait for a handshake.
	extra, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer extra.Close()
	buf := make([]byte, 1)
	if _, err := extra.Read(buf); err == nil {
		t.Error("a connection over the bound is served")
	}
	if n := l.unauthenticated.Load(); n != maxUnauthenticated {
		t.Errorf("unauthenticated sessions: %d", n)
	}

	for _, c := range conns {
		_ = c.Close()
	}
	conns = nil
	waitFor(t, "the closed connections to make room", func() bool { return l.unauthenticated.Load() == 0 })
	if _, _, err := n.attach(context.Background(), n.ticket("n1"), "a\nb\n"); err != nil {
		t.Errorf("a session after the bound was reached: %s", err)
	}
}

type noFileConn struct{ net.Conn }

// A connection with no descriptor a session can inherit is refused.
func TestConnFileRefusesAConnectionWithNoDescriptor(t *testing.T) {
	c, _ := net.Pipe()
	defer c.Close()
	if _, err := connFile(noFileConn{c}); err == nil {
		t.Error("expected a refusal")
	}
}

// A connection known only by its descriptor, as a vsock one is, is duplicated
// for the session, and the copy serves as the original does.
func TestConnFileOfASyscallConn(t *testing.T) {
	listener, err := net.Listen("unix", filepath.Join(t.TempDir(), "s"))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	client, err := net.Dial("unix", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	server, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	f, err := connFile(server)
	if err != nil {
		t.Fatal(err)
	}
	_ = server.Close()
	conn, err := inheritedConn(f)
	_ = f.Close()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := client.Write([]byte("hi")); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 2)
	if _, err := io.ReadFull(conn, b); err != nil || string(b) != "hi" {
		t.Errorf("got %q, %v", b, err)
	}
}

// A listener given is the one accepted on.
func TestNewListenerOn(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if l := NewListenerOn(listener); l.listener != listener {
		t.Error("the listener given is not kept")
	}
}
