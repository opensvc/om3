package object

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/testhelper"
)

// testCert returns a certificate for names expiring at notAfter, signed by a
// test issuer, or self-signed.
func testCert(t *testing.T, names []string, notAfter time.Time, selfSigned bool) []byte {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: names[0]},
		DNSNames:     names,
		NotBefore:    notAfter.Add(-90 * 24 * time.Hour),
		NotAfter:     notAfter,
	}
	parent, parentPriv := tmpl, priv
	if !selfSigned {
		caPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		parent = &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Test CA"}, IsCA: true, BasicConstraintsValid: true, NotAfter: notAfter.Add(time.Hour)}
		parentPriv = caPriv
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, &priv.PublicKey, parentPriv)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

// A certificate is issued when there is none, when the current one is
// self-signed, names other domains, or expires within the renewal window,
// and not otherwise: a renewal running on a schedule does nothing most days.
func TestAcmeRenewalDue(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	before := 30 * 24 * time.Hour
	domains := []string{"dev2-oc3.opensvc.com"}
	for _, tc := range []struct {
		name string
		cert []byte
		due  bool
	}{
		{"no certificate", nil, true},
		{"not readable", []byte("garbage"), true},
		{"valid for 60 days", testCert(t, domains, now.Add(60*24*time.Hour), false), false},
		{"expiring in 20 days", testCert(t, domains, now.Add(20*24*time.Hour), false), true},
		{"expired", testCert(t, domains, now.Add(-time.Hour), false), true},
		{"self-signed, as made by certificate create", testCert(t, domains, now.Add(300*24*time.Hour), true), true},
		{"naming another domain", testCert(t, []string{"other.opensvc.com"}, now.Add(60*24*time.Hour), false), true},
		{"naming the domains in another order", testCert(t, []string{"b.opensvc.com", "a.opensvc.com"}, now.Add(60*24*time.Hour), false), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := domains
			if tc.name == "naming the domains in another order" {
				want = []string{"a.opensvc.com", "b.opensvc.com"}
			}
			due, reason := acmeRenewalDue(tc.cert, want, now, before)
			assert.Equal(t, tc.due, due, reason)
			assert.NotEmpty(t, reason)
		})
	}
}

// A sec naming no ACME directory has its certificate generated as certificate
// create does, self-signed here, when due: a second renewal does nothing, a
// forced one generates it anew with the same private key.
func TestRenewGeneratedCertificate(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := SetClusterConfig()
	require.NoError(t, err)

	p := naming.Path{Name: "web", Kind: naming.KindSec, Namespace: "ns1"}
	o, err := NewSec(p, WithConfigData([]byte("[DEFAULT]\ncn = web.example.com\nalt_names = web.example.com www.example.com\nbits = 2048\n")))
	require.NoError(t, err)
	r, err := o.RenewCertificate(context.Background(), CertificateRenewOptions{})
	require.NoError(t, err)
	assert.True(t, r.Renewed, r.Reason)
	assert.Equal(t, "no certificate", r.Reason)
	assert.Empty(t, r.Directory)
	key1, err := o.DecodeKey("private_key")
	require.NoError(t, err)

	r, err = o.RenewCertificate(context.Background(), CertificateRenewOptions{})
	require.NoError(t, err)
	assert.False(t, r.Renewed, "a self-signed certificate is what the sec asks: not due")

	r, err = o.RenewCertificate(context.Background(), CertificateRenewOptions{Force: true})
	require.NoError(t, err)
	assert.True(t, r.Renewed)
	key2, err := o.DecodeKey("private_key")
	require.NoError(t, err)
	assert.Equal(t, key1, key2, "the private key is kept")
}

func TestAcmeDirectoryAliases(t *testing.T) {
	assert.Equal(t, AcmeLetsEncrypt, acmeDirectory("letsencrypt"))
	assert.Equal(t, AcmeLetsEncryptStaging, acmeDirectory("letsencrypt-staging"))
	assert.Equal(t, "https://acme.example/dir", acmeDirectory("https://acme.example/dir"))
	assert.Equal(t, "", acmeDirectory(""))
}
