package tlssecs

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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/plog"
)

// writeCert writes a self-signed certificate for name and its key in the
// files, and returns the certificate.
func writeCert(t *testing.T, certFile, keyFile, name string) []byte {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: name},
		DNSNames:     []string{name},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	require.NoError(t, err)
	keyDer, err := x509.MarshalPKCS8PrivateKey(priv)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDer}), 0600))
	return der
}

// The certificate of system/sec/cert is presented from its files, read again
// when they change, as when the certificate is renewed: the listener presents
// the new one without a restart.
func TestDefaultCertificateFollowsTheFiles(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	store := New(plog.NewDefaultLogger()).WithDefault(certFile, keyFile)

	first := writeCert(t, certFile, keyFile, "node1")
	cert, err := store.GetCertificate(&tls.ClientHelloInfo{})
	require.NoError(t, err)
	require.NotNil(t, cert)
	assert.Equal(t, first, cert.Certificate[0])

	// A later modification time, whatever the resolution of the clock.
	later := time.Now().Add(time.Second)
	renewed := writeCert(t, certFile, keyFile, "node2")
	require.NoError(t, os.Chtimes(certFile, later, later))
	require.NoError(t, os.Chtimes(keyFile, later, later))
	cert, err = store.GetCertificate(&tls.ClientHelloInfo{})
	require.NoError(t, err)
	require.NotNil(t, cert)
	assert.Equal(t, renewed, cert.Certificate[0], "the renewed certificate is presented")
}

// Without files, the listener presents the certificate it loaded when it
// started.
func TestDefaultCertificateWithoutFiles(t *testing.T) {
	store := New(plog.NewDefaultLogger())
	cert, err := store.GetCertificate(&tls.ClientHelloInfo{})
	require.NoError(t, err)
	assert.Nil(t, cert)
}

// presented returns the certificate a server whose only source of
// certificates is the store presents to a client asking no name, as one
// dialing an ip address.
func presented(t *testing.T, store *Store) []byte {
	t.Helper()
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	go func() {
		srv := tls.Server(server, &tls.Config{GetCertificate: store.GetCertificate})
		_ = srv.Handshake()
	}()
	cli := tls.Client(client, &tls.Config{InsecureSkipVerify: true})
	require.NoError(t, cli.Handshake())
	return cli.ConnectionState().PeerCertificates[0].Raw
}

// A client asking no name is presented the renewed certificate too: the
// listener has no certificate of its own, loaded once at its start.
func TestHandshakeWithoutServerNameFollowsTheFiles(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	store := New(plog.NewDefaultLogger()).WithDefault(certFile, keyFile)

	first := writeCert(t, certFile, keyFile, "node1")
	assert.Equal(t, first, presented(t, store))

	later := time.Now().Add(time.Second)
	renewed := writeCert(t, certFile, keyFile, "node2")
	require.NoError(t, os.Chtimes(certFile, later, later))
	require.NoError(t, os.Chtimes(keyFile, later, later))
	assert.Equal(t, renewed, presented(t, store), "the renewed certificate is presented")
}
