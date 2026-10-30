//go:build linux

package console

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"testing"
	"time"
)

// fileLikeConn fails its reads past a deadline as a file does, with a path
// error, and goes through connError as a vsock connection does.
type fileLikeConn struct {
	net.Conn
	wrap bool
}

func (t *fileLikeConn) Read(b []byte) (int, error) {
	n, err := t.Conn.Read(b)
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		err = &os.PathError{Op: "read", Path: "vsock", Err: os.ErrDeadlineExceeded}
	}
	if t.wrap {
		err = connError(err)
	}
	return n, err
}

func selfSigned(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{SerialNumber: big.NewInt(1), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// readAfterTimeout does what the http server does when a websocket takes a
// connection over: a read interrupted by a past deadline, the deadline
// cleared, and the reads going on.
func readAfterTimeout(t *testing.T, wrap bool) error {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		c, err := l.Accept()
		if err != nil {
			return
		}
		client := tls.Client(c, &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS13}) // #nosec G402
		defer client.Close()
		if err := client.Handshake(); err != nil {
			return
		}
		time.Sleep(200 * time.Millisecond)
		_, _ = fmt.Fprint(client, "hi")
		time.Sleep(200 * time.Millisecond)
	}()
	c, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	server := tls.Server(&fileLikeConn{Conn: c, wrap: wrap}, &tls.Config{
		Certificates: []tls.Certificate{selfSigned(t)},
		MinVersion:   tls.VersionTLS13,
	})
	if err := server.Handshake(); err != nil {
		t.Fatal(err)
	}
	_ = server.SetReadDeadline(time.Now().Add(time.Millisecond))
	time.Sleep(10 * time.Millisecond)
	if _, err := server.Read(make([]byte, 2)); err == nil {
		t.Fatal("expected the read to time out")
	}
	_ = server.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, err = io.ReadFull(server, make([]byte, 2))
	return err
}

func TestAReadInterruptedByADeadlineDoesNotEndTheSession(t *testing.T) {
	if err := readAfterTimeout(t, true); err != nil {
		t.Errorf("the connection did not survive the deadline: %s", err)
	}
	if err := readAfterTimeout(t, false); err == nil {
		t.Error("a path error should be what breaks the tls layer, this test proves nothing")
	}
}
