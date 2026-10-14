package nmon

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/plog"
)

// genCert issues the certificate of system/sec/cert for the alt names.
func genCert(t *testing.T, altNames string) {
	t.Helper()
	sec, err := object.NewSec(naming.SecCert, object.WithConfigData([]byte("[DEFAULT]\nalt_names = "+altNames+"\n")))
	require.NoError(t, err)
	require.NoError(t, sec.GenCert())
}

// A node the certificate of system/sec/cert does not name has an issue,
// said when it appears, and gone once a certificate names it.
func TestCheckCert(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)
	m := &Manager{localhost: "node1", log: plog.NewDefaultLogger()}

	m.checkCert()
	assert.Empty(t, m.certIssues, "no system/sec/cert yet")

	genCert(t, "node1 node2")
	m.checkCert()
	assert.Empty(t, m.certIssues, "named")

	genCert(t, "vip.acme.com")
	m.checkCert()
	require.Len(t, m.certIssues, 1)
	assert.Contains(t, m.certIssues[0], "does not name node1")
	assert.Contains(t, m.certIssues[0], "vip.acme.com")
	assert.Equal(t, m.certIssues, m.nodeIssues(m.nodeConfig))

	genCert(t, "NODE1")
	m.checkCert()
	assert.Empty(t, m.certIssues, "names are not case sensitive")
	assert.Empty(t, m.nodeIssues(m.nodeConfig))

	// The listener does not load a certificate with the private key of
	// another: it keeps presenting the one it had.
	other, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalECPrivateKey(other)
	require.NoError(t, err)
	sec, err := object.NewSec(naming.SecCert)
	require.NoError(t, err)
	require.NoError(t, sec.ChangeKey("private_key", pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})))
	m.checkCert()
	require.Len(t, m.certIssues, 1)
	assert.Contains(t, m.certIssues[0], "are not a pair")
}

// The certificates of system/sec/cert and system/sec/ca are issues within
// their warning window before their expiry, and once expired.
func TestCheckCertExpiry(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)
	m := &Manager{localhost: "node1", log: plog.NewDefaultLogger()}

	ca, err := object.NewSec(naming.SecCa, object.WithConfigData([]byte("[DEFAULT]\nvalidity = 1y\n")))
	require.NoError(t, err)
	require.NoError(t, ca.GenCert())
	cert, err := object.NewSec(naming.SecCert, object.WithConfigData([]byte("[DEFAULT]\nca = system/sec/ca\nalt_names = node1\nvalidity = 60d\n")))
	require.NoError(t, err)
	require.NoError(t, cert.GenCert())

	now := time.Now()
	assert.Empty(t, m.certIssuesNow(now))

	l := m.certIssuesNow(now.Add(50 * 24 * time.Hour))
	require.Len(t, l, 1)
	assert.Contains(t, l[0], "system/sec/cert certificate expires on")

	l = m.certIssuesNow(now.Add(61 * 24 * time.Hour))
	require.Len(t, l, 1)
	assert.Contains(t, l[0], "system/sec/cert certificate expired on")
	assert.Contains(t, l[0], "the peers refuse it now")

	l = m.certIssuesNow(now.Add(300 * 24 * time.Hour))
	require.Len(t, l, 2)
	assert.Contains(t, l[1], "system/sec/ca certificate expires on")
	assert.Contains(t, l[1], "certificate renew --force")
}
