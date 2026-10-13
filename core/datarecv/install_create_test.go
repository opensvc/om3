package datarecv

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/hostname"
)

// A sec an install line creates for a key of its certificate has a
// self-signed one, which the line installs: what reads the file has one to
// start with.
func TestASecCreatedForItsCertificateHasOne(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)

	p := naming.Path{Name: "gw", Kind: naming.KindSvc, Namespace: naming.NsRoot}
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte("[DEFAULT]\nnodes = "+hostname.Hostname()+"\n"), 0600))
	o, err := object.NewCore(p)
	require.NoError(t, err)

	head := t.TempDir()
	recv := &DataRecv{
		to:      headReceiver{o: o, head: head},
		Install: []string{"/default.pem", "from", "./sec/gw", "key", "fullpem"},
	}
	_, err = recv.install(context.Background())
	require.NoError(t, err)

	b, err := os.ReadFile(filepath.Join(head, "default.pem"))
	require.NoError(t, err)
	assert.Contains(t, string(b), "-----BEGIN CERTIFICATE-----")
	assert.Contains(t, string(b), "PRIVATE KEY-----")
}
