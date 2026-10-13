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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
