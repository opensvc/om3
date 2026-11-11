package httpclientcache

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// newCAServer returns a tls server whose certificate a new authority issues
// for the name only, and the file of that authority.
func newCAServer(t *testing.T, name string) (*httptest.Server, string) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	caCert, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: name},
		DNSNames:     []string{name},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	require.NoError(t, err)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	caFile := filepath.Join(t.TempDir(), "ca")
	require.NoError(t, os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0o600))
	return srv, caFile
}

func get(t *testing.T, o Options, url string) error {
	t.Helper()
	cli, err := Client(o)
	require.NoError(t, err)
	resp, err := cli.Get(url)
	if err == nil {
		_ = resp.Body.Close()
	}
	return err
}

func TestClientVerifiesServer(t *testing.T) {
	srv, caFile := newCAServer(t, "node1")

	t.Run("without client certificate nor authority", func(t *testing.T) {
		require.Error(t, get(t, Options{}, srv.URL), "an unknown authority must be refused")
	})
	t.Run("insecure", func(t *testing.T) {
		require.NoError(t, get(t, Options{InsecureSkipVerify: true}, srv.URL))
	})
	t.Run("authority, address not named by the certificate", func(t *testing.T) {
		require.Error(t, get(t, Options{RootCA: caFile}, srv.URL))
	})
	t.Run("authority and server name", func(t *testing.T) {
		require.NoError(t, get(t, Options{RootCA: caFile, ServerName: "node1"}, srv.URL))
	})
	t.Run("authority and wrong server name", func(t *testing.T) {
		require.Error(t, get(t, Options{RootCA: caFile, ServerName: "node2"}, srv.URL))
	})
}

func TestClientCachedByAuthorityContent(t *testing.T) {
	srv1, caFile1 := newCAServer(t, "node1")
	srv2, caFile2 := newCAServer(t, "node1")
	o := Options{RootCA: caFile1, ServerName: "node1"}

	c1, err := Client(o)
	require.NoError(t, err)
	c2, err := Client(o)
	require.NoError(t, err)
	require.Same(t, c1, c2, "the clients of the same authority are cached")

	// The file now holds another authority: the client of the first one
	// must not be served for it.
	b, err := os.ReadFile(caFile2)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(caFile1, b, 0o600))
	require.NoError(t, get(t, o, srv2.URL))
	require.Error(t, get(t, o, srv1.URL))
}

// silentListener accepts the connections and never answers on them, as a
// node whose daemon is stuck. The connections are closed with the listener.
func silentListener(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	var (
		mu    sync.Mutex
		conns []net.Conn
		done  = make(chan struct{})
	)
	go func() {
		defer close(done)
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns = append(conns, c)
			mu.Unlock()
		}
	}()
	t.Cleanup(func() {
		_ = ln.Close()
		<-done
		for _, c := range conns {
			_ = c.Close()
		}
	})
	return ln
}

// A node that accepts the connection and never answers the handshake: the
// request ends at tlsHandshakeTimeout, not at the client timeout, which is
// zero here.
func TestClientTLSHandshakeTimeout(t *testing.T) {
	ln := silentListener(t)

	saved := tlsHandshakeTimeout
	tlsHandshakeTimeout = 200 * time.Millisecond
	t.Cleanup(func() { tlsHandshakeTimeout = saved })

	cli, err := newClient(Options{InsecureSkipVerify: true})
	require.NoError(t, err)

	begin := time.Now()
	_, err = cli.Get("https://" + ln.Addr().String())
	require.Error(t, err)
	require.Less(t, time.Since(begin), 2*time.Second)
	require.Contains(t, err.Error(), "TLS handshake timeout")
}

// The connect of a client is bounded by dialTimeout, not by the kernel. A
// timeout already elapsed fails the connect to any address, a reachable one
// included, which an unreachable one would need minutes to show.
func TestClientDialTimeout(t *testing.T) {
	ln := silentListener(t)

	saved := dialTimeout
	dialTimeout = time.Nanosecond
	t.Cleanup(func() { dialTimeout = saved })

	cli, err := newClient(Options{InsecureSkipVerify: true})
	require.NoError(t, err)

	_, err = cli.Get("https://" + ln.Addr().String())
	require.Error(t, err)
	var netErr net.Error
	require.ErrorAs(t, err, &netErr)
	require.True(t, netErr.Timeout(), "%s", err)
	require.Contains(t, err.Error(), "dial", "the connect times out")
}

// The transport of the nodes drops its idle connections, and speaks
// HTTP/1.1 as before: no clone of http.DefaultTransport attempting HTTP/2.
func TestNewTransport(t *testing.T) {
	tp := NewTransport(&tls.Config{})
	require.NotNil(t, tp.DialContext, "the connect is bounded")
	require.Equal(t, tlsHandshakeTimeout, tp.TLSHandshakeTimeout)
	require.Equal(t, idleConnTimeout, tp.IdleConnTimeout)
	require.False(t, tp.ForceAttemptHTTP2)
	require.Nil(t, tp.Proxy, "the nodes are reached with no proxy")
}
