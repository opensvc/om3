package switchbrocade

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/testhelper"
)

// fakeSwitch is an ssh server answering the brocade commands, as a switch
// would. Its zoneshow is only found in a login shell, as on the firmwares
// the v2 driver met.
type fakeSwitch struct {
	addr     string
	hostKey  ssh.Signer
	listener net.Listener
	wg       sync.WaitGroup

	mu   sync.Mutex
	cmds []string
}

var fakeOutputs = map[string]string{
	"switchshow":               "switchName:\tsansw1\n",
	"nsshow":                   "{\n}\n",
	"bash --login -c zoneshow": "Effective configuration:\n",
}

func newSigner(t *testing.T) (ssh.Signer, ed25519.PrivateKey) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return signer, priv
}

func startFakeSwitch(t *testing.T, clientKey ssh.PublicKey) *fakeSwitch {
	t.Helper()
	hostKey, _ := newSigner(t)
	config := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if conn.User() == "admin" && string(key.Marshal()) == string(clientKey.Marshal()) {
				return nil, nil
			}
			return nil, os.ErrPermission
		},
	}
	config.AddHostKey(hostKey)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &fakeSwitch{addr: l.Addr().String(), hostKey: hostKey, listener: l}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				s.serve(conn, config)
			}()
		}
	}()
	t.Cleanup(func() {
		_ = l.Close()
		s.wg.Wait()
	})
	return s
}

func (s *fakeSwitch) serve(conn net.Conn, config *ssh.ServerConfig) {
	defer conn.Close()
	_, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	go ssh.DiscardRequests(reqs)
	for newChan := range chans {
		ch, requests, err := newChan.Accept()
		if err != nil {
			return
		}
		for req := range requests {
			if req.Type != "exec" {
				_ = req.Reply(false, nil)
				continue
			}
			_ = req.Reply(true, nil)
			cmd := string(req.Payload[4:])
			s.mu.Lock()
			s.cmds = append(s.cmds, cmd)
			s.mu.Unlock()
			status := uint32(0)
			if out, ok := fakeOutputs[cmd]; ok {
				_, _ = ch.Write([]byte(out))
			} else {
				_, _ = ch.Stderr().Write([]byte("rbash: " + cmd + ": command not found\n"))
				status = 127
			}
			b := make([]byte, 4)
			binary.BigEndian.PutUint32(b, status)
			_, _ = ch.SendRequest("exit-status", false, b)
			_ = ch.Close()
			break
		}
	}
}

// newTestSwitch returns the driver of a switch section pointing at the fake
// switch, logging in with the key file.
func newTestSwitch(t *testing.T, s *fakeSwitch, keyFile, method string) *T {
	t.Helper()
	host, port, _ := net.SplitHostPort(s.addr)
	sshPort = port
	t.Cleanup(func() { sshPort = "22" })
	conf := "[switch#sansw1]\ntype = brocade\nname = " + host + "\nusername = admin\nkey = " + keyFile + "\n"
	if method != "" {
		conf += "method = " + method + "\n"
	}
	// The keywords are evaluated as the node declares them, so the section
	// is written in the node configuration of a test root.
	testhelper.Setup(t)
	if err := os.WriteFile(rawconfig.NodeConfigFile(), []byte(conf), 0600); err != nil {
		t.Fatal(err)
	}
	n, err := object.NewNode()
	if err != nil {
		t.Fatal(err)
	}
	drv := &T{}
	drv.SetName("switch#sansw1")
	drv.SetConfig(n.MergedConfig())
	return drv
}

func writeKeyFile(t *testing.T, priv ed25519.PrivateKey) string {
	t.Helper()
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(p, pem.EncodeToMemory(block), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

// The report holds the output of each command, zoneshow read through a
// login shell after the plain command was not found, and the key of the
// switch is trusted on this first connection.
func TestReport(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	clientSigner, clientPriv := newSigner(t)
	s := startFakeSwitch(t, clientSigner.PublicKey())
	drv := newTestSwitch(t, s, writeKeyFile(t, clientPriv), "")

	data, err := drv.Report(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"switchshow": "switchName:\tsansw1\n",
		"nsshow":     "{\n}\n",
		"zoneshow":   "Effective configuration:\n",
	}
	for k, v := range want {
		if data[k] != v {
			t.Errorf("%s: got %q, want %q", k, data[k], v)
		}
	}
	got := strings.Join(s.cmds, ",")
	if got != "switchshow,nsshow,zoneshow,bash --login -c zoneshow" {
		t.Errorf("commands run: %s", got)
	}

	// The same switch is trusted again.
	if _, err := drv.Report(context.Background()); err != nil {
		t.Fatalf("second report: %s", err)
	}
}

// A switch presenting another key than the one trusted is refused.
func TestReportRefusesAChangedHostKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	clientSigner, clientPriv := newSigner(t)
	keyFile := writeKeyFile(t, clientPriv)

	first := startFakeSwitch(t, clientSigner.PublicKey())
	if _, err := newTestSwitch(t, first, keyFile, "").Report(context.Background()); err != nil {
		t.Fatal(err)
	}
	_ = first.listener.Close()

	// Another server on the same address, as a switch replaced or an
	// impostor would be.
	second := startFakeSwitchAt(t, clientSigner.PublicKey(), first.addr)
	if second == nil {
		t.Skip("the address of the first server could not be reused")
	}
	if _, err := newTestSwitch(t, second, keyFile, "").Report(context.Background()); err == nil {
		t.Fatal("a changed host key is accepted")
	}
}

func startFakeSwitchAt(t *testing.T, clientKey ssh.PublicKey, addr string) *fakeSwitch {
	t.Helper()
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil
	}
	_ = l.Close()
	hostKey, _ := newSigner(t)
	config := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			return nil, nil
		},
	}
	config.AddHostKey(hostKey)
	l, err = net.Listen("tcp", addr)
	if err != nil {
		return nil
	}
	s := &fakeSwitch{addr: addr, hostKey: hostKey, listener: l}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				s.serve(conn, config)
			}()
		}
	}()
	t.Cleanup(func() {
		_ = l.Close()
		s.wg.Wait()
	})
	return s
}

// The telnet method v2 offered is refused with a message saying what to
// set instead.
func TestReportRefusesTelnet(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	clientSigner, clientPriv := newSigner(t)
	s := startFakeSwitch(t, clientSigner.PublicKey())
	_, err := newTestSwitch(t, s, writeKeyFile(t, clientPriv), "telnet").Report(context.Background())
	if err == nil || !strings.Contains(err.Error(), "method = ssh") {
		t.Fatalf("telnet not refused as expected: %v", err)
	}
}

// The collector knows a switch by the name it is reached by, without the
// port a port forward adds.
func TestReportName(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"sansw1.my.corp", "sansw1.my.corp"},
		{"sansw1.my.corp:2222", "sansw1.my.corp"},
		{"", "sansw1"},
	} {
		testhelper.Setup(t)
		conf := "[switch#sansw1]\ntype = brocade\nusername = admin\n"
		if tc.name != "" {
			conf += "name = " + tc.name + "\n"
		}
		if err := os.WriteFile(rawconfig.NodeConfigFile(), []byte(conf), 0600); err != nil {
			t.Fatal(err)
		}
		n, err := object.NewNode()
		if err != nil {
			t.Fatal(err)
		}
		drv := &T{}
		drv.SetName("switch#sansw1")
		drv.SetConfig(n.MergedConfig())
		if got := drv.ReportName(); got != tc.want {
			t.Errorf("name %q: got %q, want %q", tc.name, got, tc.want)
		}
	}
}
