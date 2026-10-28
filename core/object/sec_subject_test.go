package object

import (
	"crypto/rand"
	"crypto/x509"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/testhelper"
)

func setupSubjectTest(t *testing.T) {
	t.Helper()
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := SetClusterConfig()
	require.NoError(t, err)
}

func certOf(t *testing.T, o Sec) *x509.Certificate {
	t.Helper()
	b, err := o.DecodeKey("certificate")
	require.NoError(t, err)
	cert, err := certFromPEM(b)
	require.NoError(t, err)
	return cert
}

// A keyword not set leaves its attribute out of the subject, rather than
// empty, which X.520 refuses.
func TestSubjectLeavesTheUnsetAttributesOut(t *testing.T) {
	setupSubjectTest(t)
	ca, err := NewSec(naming.Path{Name: "ca", Kind: naming.KindSec, Namespace: "ns1"}, WithConfigData([]byte("[DEFAULT]\ncn = cluster1 CA\no = OpenSVC\nbits = 2048\n")))
	require.NoError(t, err)
	require.NoError(t, ca.GenCert())
	assert.Equal(t, "CN=cluster1 CA,O=OpenSVC", certOf(t, ca).Subject.String())

	web, err := NewSec(naming.Path{Name: "web", Kind: naming.KindSec, Namespace: "ns1"}, WithConfigData([]byte("[DEFAULT]\nca = ns1/sec/ca\ncn = web.example.com\nbits = 2048\n")))
	require.NoError(t, err)
	require.NoError(t, web.GenCert())
	cert := certOf(t, web)
	assert.Equal(t, "CN=web.example.com", cert.Subject.String())
	assert.Equal(t, "CN=cluster1 CA,O=OpenSVC", cert.Issuer.String())
}

// A ca issued before the unset attributes were left out keeps its subject,
// empty attributes included, when renewed with the keywords it was issued
// with: it is the issuer of the certificates it signed. A keyword changed
// since gives the renewal the subject it says.
func TestRenewKeepsALegacySubject(t *testing.T) {
	setupSubjectTest(t)
	o, err := NewSec(naming.Path{Name: "ca", Kind: naming.KindSec, Namespace: "ns1"}, WithConfigData([]byte("[DEFAULT]\nbits = 2048\n")))
	require.NoError(t, err)
	s := o

	// The certificate an older agent issued, with each attribute empty.
	priv, err := s.getPriv()
	require.NoError(t, err)
	tmpl := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               s.legacySubject(),
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	require.NoError(t, err)
	require.NoError(t, s.setCert(der))
	legacy := certOf(t, o).RawSubject
	require.Equal(t, []string{""}, certOf(t, o).Subject.Country, "the legacy subject holds empty attributes")

	require.NoError(t, o.GenCert())
	cert := certOf(t, o)
	assert.Equal(t, legacy, cert.RawSubject, "the renewal keeps the subject")
	assert.Equal(t, legacy, cert.RawIssuer, "and issues itself with it")

	require.NoError(t, s.config.Set(*keyop.Parse("cn=cluster1 CA")))
	require.NoError(t, o.GenCert())
	assert.Equal(t, "CN=cluster1 CA", certOf(t, o).Subject.String(), "a keyword changed gives the subject it says")
}
