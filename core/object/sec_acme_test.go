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
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/key"
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

// A certificate the sec did not issue, as one installed from elsewhere, names
// what it was installed for, never alt_names: a renewal keeps it, and a
// forced one only replaces it.
func TestRenewKeepsACertificateTheSecDidNotIssue(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := SetClusterConfig()
	require.NoError(t, err)

	p := naming.Path{Name: "web", Kind: naming.KindSec, Namespace: "ns1"}
	o, err := NewSec(p, WithConfigData([]byte("[DEFAULT]\nalt_names = node1\nbits = 2048\n")))
	require.NoError(t, err)
	installed := testCert(t, []string{"*.example.com"}, time.Now().Add(365*24*time.Hour), false)
	require.NoError(t, o.AddKey("certificate", installed))

	_, err = o.RenewCertificate(context.Background(), CertificateRenewOptions{})
	require.ErrorIs(t, err, ErrCertificateNotIssued)
	current, err := o.DecodeKey("certificate")
	require.NoError(t, err)
	assert.Equal(t, installed, current, "the installed certificate is kept")

	r, err := o.RenewCertificate(context.Background(), CertificateRenewOptions{Force: true})
	require.NoError(t, err)
	assert.True(t, r.Renewed, "a forced renewal replaces it")
}

// A certificate issued for alt_names = {clusternodes} by the ca of the sec
// names the cluster nodes, and is renewed when they change.
func TestRenewNamesTheClusterNodes(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := SetClusterConfig()
	require.NoError(t, err)

	caPath := naming.Path{Name: "ca", Kind: naming.KindSec, Namespace: "ns1"}
	ca, err := NewSec(caPath, WithConfigData([]byte("[DEFAULT]\nbits = 2048\n")))
	require.NoError(t, err)
	require.NoError(t, ca.GenCert())

	p := naming.Path{Name: "cert", Kind: naming.KindSec, Namespace: "ns1"}
	o, err := NewSec(p, WithConfigData([]byte("[DEFAULT]\nca = ns1/sec/ca\nalt_names = {clusternodes}\nbits = 2048\n")))
	require.NoError(t, err)
	names := func() []string {
		b, err := o.DecodeKey("certificate")
		require.NoError(t, err)
		cert, err := certFromPEM(b)
		require.NoError(t, err)
		return cert.DNSNames
	}

	r, err := o.RenewCertificate(context.Background(), CertificateRenewOptions{})
	require.NoError(t, err)
	assert.True(t, r.Renewed, r.Reason)
	assert.Equal(t, []string{"node1"}, names())

	r, err = o.RenewCertificate(context.Background(), CertificateRenewOptions{})
	require.NoError(t, err)
	assert.False(t, r.Renewed, "the certificate names the cluster nodes: not due")

	cluster, err := NewCluster(WithVolatile(false))
	require.NoError(t, err)
	require.NoError(t, cluster.Config().Set(*keyop.New(key.New("cluster", "nodes"), keyop.Set, "node1 node2", 0)))
	_, err = SetClusterConfig()
	require.NoError(t, err)

	// The daemon reads the sec anew for each renewal, as here: the
	// reference is evaluated with the cluster nodes of the time.
	o, err = NewSec(p, WithVolatile(false))
	require.NoError(t, err)
	r, err = o.RenewCertificate(context.Background(), CertificateRenewOptions{})
	require.NoError(t, err)
	assert.True(t, r.Renewed, "a node joined: due")
	assert.Equal(t, []string{"node1", "node2"}, names())
}

// A generated certificate is renewed when alt_names changes, an ip address
// as well as a name: a certificate issued for the alt_names of the time is
// not due anymore.
func TestRenewFollowsAltNames(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := SetClusterConfig()
	require.NoError(t, err)

	p := naming.Path{Name: "web", Kind: naming.KindSec, Namespace: "ns1"}
	o, err := NewSec(p, WithConfigData([]byte("[DEFAULT]\nalt_names = node1\nbits = 2048\n")))
	require.NoError(t, err)
	r, err := o.RenewCertificate(context.Background(), CertificateRenewOptions{})
	require.NoError(t, err)
	require.True(t, r.Renewed, r.Reason)
	r, err = o.RenewCertificate(context.Background(), CertificateRenewOptions{})
	require.NoError(t, err)
	require.False(t, r.Renewed, r.Reason)

	for _, tc := range []struct {
		altNames string
		dns      []string
		ips      []string
	}{
		// An ip address is a dns name too, as the certificates are
		// generated since 2022.
		{"node1 10.0.0.1", []string{"node1", "10.0.0.1"}, []string{"127.0.0.1", "10.0.0.1"}},
		{"node1 node2 10.0.0.1", []string{"node1", "node2", "10.0.0.1"}, []string{"127.0.0.1", "10.0.0.1"}},
		{"node2", []string{"node2"}, []string{"127.0.0.1"}},
	} {
		require.NoError(t, o.Config().Set(*keyop.New(key.New("DEFAULT", "alt_names"), keyop.Set, tc.altNames, 0)))
		o, err = NewSec(p, WithVolatile(false))
		require.NoError(t, err)
		r, err := o.RenewCertificate(context.Background(), CertificateRenewOptions{})
		require.NoError(t, err)
		assert.True(t, r.Renewed, "alt_names = %s: %s", tc.altNames, r.Reason)
		b, err := o.DecodeKey("certificate")
		require.NoError(t, err)
		cert, err := certFromPEM(b)
		require.NoError(t, err)
		assert.Equal(t, tc.dns, cert.DNSNames)
		ips := make([]string, len(cert.IPAddresses))
		for i, ip := range cert.IPAddresses {
			ips[i] = ip.String()
		}
		assert.Equal(t, tc.ips, ips)
	}
}

// A certificate whose ip addresses are not the ones of alt_names is due, its
// dns names alike.
func TestCertificateIPsDue(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		DNSNames:     []string{"node1"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("10.0.0.1")},
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	require.NoError(t, err)
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	due, _ := certificateIPsDue(cert, []net.IP{net.ParseIP("10.0.0.1"), net.ParseIP("127.0.0.1")}, "valid")
	assert.False(t, due, "the same ip addresses, in another order")
	due, reason := certificateIPsDue(cert, []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("10.0.0.2")}, "valid")
	assert.True(t, due)
	assert.Contains(t, reason, "10.0.0.2")
}
