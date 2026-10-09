package listener

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/plog"
)

func certNamesOf(t *testing.T) []string {
	t.Helper()
	sec, err := object.NewSec(certPath, object.WithVolatile(true))
	require.NoError(t, err)
	b, err := sec.DecodeKey("certificate")
	require.NoError(t, err)
	cert, err := certFromPEMForTest(b)
	require.NoError(t, err)
	return cert.DNSNames
}

// A system/sec/cert bootstrapped before alt_names named the cluster nodes, as
// the one of dev1, has its certificate valid for 127.0.0.1 only: the speaker
// sets alt_names = {clusternodes} and renews it for the cluster nodes.
func TestRenewCertNamesTheClusterNodes(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)

	ca, err := object.NewSec(caPath, object.WithVolatile(false))
	require.NoError(t, err)
	require.NoError(t, ca.GenCert())
	sec, err := object.NewSec(certPath, object.WithConfigData([]byte("[DEFAULT]\nca = "+caPath.String()+"\n")))
	require.NoError(t, err)
	require.NoError(t, sec.GenCert())
	assert.Empty(t, certNamesOf(t), "no name, as on dev1")

	lsnr := &T{log: plog.NewDefaultLogger()}
	lsnr.renewCert(context.Background())

	b, err := os.ReadFile(certPath.ConfigFile())
	require.NoError(t, err)
	assert.Contains(t, string(b), "alt_names = "+certAltNames, "the reference is set, not its value")
	assert.Equal(t, []string{"node1"}, certNamesOf(t))
}

func certFromPEMForTest(b []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, fmt.Errorf("no pem block")
	}
	return x509.ParseCertificate(block.Bytes)
}
