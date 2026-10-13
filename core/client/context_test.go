package client

import (
	"context"
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
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/clientcontext"
)

// newPrivateCAServer returns a tls server whose certificate a private
// authority issues for 127.0.0.1, and the pem of that authority.
func newPrivateCAServer(t *testing.T) (*httptest.Server, []byte) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "cluster ca"},
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
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
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
	return srv, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
}

// A context authenticating by token verifies the server by the authority it
// names, which no system one is.
func TestContextCertificateAuthority(t *testing.T) {
	srv, caPEM := newPrivateCAServer(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	require.NoError(t, os.WriteFile(filepath.Join(home, "ca.pem"), caPEM, 0o600))

	// use saves a context of the cluster, and selects it.
	use := func(name string, cluster clientcontext.Cluster) {
		cfg, err := clientcontext.Load()
		require.NoError(t, err)
		require.NoError(t, cfg.AddCluster(name, cluster))
		require.NoError(t, cfg.AddContext(name, clientcontext.Relation{ClusterRefName: name}))
		require.NoError(t, cfg.Save())
		t.Setenv("OSVC_CONTEXT", name)
	}
	get := func() error {
		c, err := New(WithBearer("token"))
		require.NoError(t, err)
		resp, err := c.GetNodePing(context.Background(), "node1")
		if err == nil {
			_ = resp.Body.Close()
		}
		return err
	}

	use("without-ca", clientcontext.Cluster{Server: srv.URL})
	require.Error(t, get(), "no authority: refused")

	ca := "~/ca.pem"
	use("with-ca", clientcontext.Cluster{Server: srv.URL, CertificateAuthority: &ca})
	require.NoError(t, get())

	// An insecure context verifies nothing: its authority file is not
	// read, and one gone is no failure.
	gone, insecure := "~/gone.pem", true
	use("insecure", clientcontext.Cluster{Server: srv.URL, CertificateAuthority: &gone, InsecureSkipVerify: &insecure})
	require.NoError(t, get())
}
