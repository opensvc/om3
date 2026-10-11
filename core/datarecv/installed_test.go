package datarecv

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
)

type headReceiver struct {
	o    any
	head string
}

func (r headReceiver) Head() string                     { return r.head }
func (r headReceiver) Log() *plog.Logger                { return plog.NewLogger(zerolog.Nop()) }
func (r headReceiver) StatusLog() resource.StatusLogger { return resource.NewStatusLog() }
func (r headReceiver) GetObject() any                   { return r.o }
func (r headReceiver) RID() string                      { return "volume#1" }

// The record of an install names each file written, the ones of a key
// pattern one by one, and is made when a required item fails after others
// were written: what reads them is to read them again.
func TestInstallRecordsTheFilesWritten(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)

	p := naming.Path{Name: "web", Kind: naming.KindSvc, Namespace: naming.NsRoot}
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte("[DEFAULT]\nnodes = "+hostname.Hostname()+"\n[volume#1]\nname = {name}\n"), 0600))
	o, err := object.NewCore(p)
	require.NoError(t, err)

	cfg := naming.Path{Name: "web", Kind: naming.KindCfg, Namespace: naming.NsRoot}
	ds, err := object.NewDataStore(cfg)
	require.NoError(t, err)
	for _, k := range []string{"a", "b1", "b2"} {
		require.NoError(t, ds.AddKey(k, []byte(k)))
	}

	recv := &DataRecv{
		to: headReceiver{o: o, head: t.TempDir()},
		Install: []string{
			"/a.conf", "from", "./cfg/web", "key", "a",
			"/conf/", "from", "./cfg/web", "key", "b*",
			"/c.conf", "from", "./cfg/web", "key", "nope", "required",
		},
	}
	_, err = recv.install(context.Background())
	require.Error(t, err, "the required item fails")

	v, err := ReadInstalled(filepath.Join(InstalledDir(p), "volume#1"))
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"/a.conf", "/conf/b1", "/conf/b2"}, v.Files)
}
