package nmon

import (
	"testing"

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
	assert.Empty(t, m.certIssueMsg, "no system/sec/cert yet")

	genCert(t, "node1 node2")
	m.checkCert()
	assert.Empty(t, m.certIssueMsg, "named")

	genCert(t, "vip.acme.com")
	m.checkCert()
	assert.Contains(t, m.certIssueMsg, "does not name node1")
	assert.Contains(t, m.certIssueMsg, "vip.acme.com")
	assert.Equal(t, []string{m.certIssueMsg}, m.nodeIssues(m.nodeConfig))

	genCert(t, "NODE1")
	m.checkCert()
	assert.Empty(t, m.certIssueMsg, "names are not case sensitive")
	assert.Empty(t, m.nodeIssues(m.nodeConfig))
}
