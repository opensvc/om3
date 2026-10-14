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

// A sec an install line creates for a key it names a source of is seeded
// from the source, and given no self-signed certificate, which would hold the
// key in its place.
func TestASecCreatedWithASourceIsSeededFromIt(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)

	p := naming.Path{Name: "gw", Kind: naming.KindSvc, Namespace: naming.NsRoot}
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte("[DEFAULT]\nnodes = "+hostname.Hostname()+"\n"), 0600))
	o, err := object.NewCore(p)
	require.NoError(t, err)

	source := filepath.Join(t.TempDir(), "gw.pem")
	require.NoError(t, os.WriteFile(source, []byte("the certificate of the source\n"), 0600))
	head := t.TempDir()
	recv := &DataRecv{
		to:      headReceiver{o: o, head: head},
		Install: []string{"/default.pem", "from", "./sec/gw", "key", "fullpem", "source", source},
	}
	_, err = recv.install(context.Background())
	require.NoError(t, err)

	b, err := os.ReadFile(filepath.Join(head, "default.pem"))
	require.NoError(t, err)
	assert.Equal(t, "the certificate of the source\n", string(b))
}
