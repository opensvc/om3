package object_test

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

// An install seeding the keys of its lines from their sources says which key
// failed, and only that one: the key it seeded first refreshes the services
// installing from the datastore, and that refresh does not run the install
// that is running, which went on to seed the next key and failed with the
// first one's name. A later write of the datastore succeeds whatever its
// consumers make of it, and its refresh seeds nothing.
func TestSeedingNamesTheKeyThatFailed(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)

	src := t.TempDir()
	okSrc := filepath.Join(src, "ok.conf")
	require.NoError(t, os.WriteFile(okSrc, []byte("ok-content"), 0600))
	missingSrc := filepath.Join(src, "dump.sql")
	head := t.TempDir()

	cfg := naming.Path{Name: "web", Kind: naming.KindCfg, Namespace: naming.NsRoot}
	ds, err := object.NewDataStore(cfg)
	require.NoError(t, err)
	require.NoError(t, ds.AddKey("other", []byte("x")), "no consumer yet")

	// Another service installs the key, with no source: the seeding of
	// web refreshes it.
	other := naming.Path{Name: "web2", Kind: naming.KindSvc, Namespace: naming.NsRoot}
	otherHead := t.TempDir()
	require.NoError(t, os.WriteFile(other.ConfigFile(), []byte("[DEFAULT]\nnodes = "+hostname.Hostname()+"\n[fs#1]\ntype = directory\npath = "+otherHead+"\n"+
		"install = /ok.conf from ./cfg/web key ok.conf\n"), 0600))

	p := naming.Path{Name: "web", Kind: naming.KindSvc, Namespace: naming.NsRoot}
	conf := "[DEFAULT]\nnodes = " + hostname.Hostname() + "\n[fs#1]\ntype = directory\npath = " + head + "\n" +
		"install = /ok.conf from ./cfg/web key ok.conf required source " + okSrc + "\n" +
		"    /dump.sql from ./cfg/web key dump.sql required source " + missingSrc + "\n"
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte(conf), 0600))
	o, err := object.NewSvc(p)
	require.NoError(t, err)
	_ = o.Resources()
	r := o.ResourceByID("fs#1")
	require.NotNil(t, r)
	installer, ok := r.(interface{ Do(context.Context) error })
	require.True(t, ok)

	err = installer.Do(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to seed key dump.sql from source")
	assert.NotContains(t, err.Error(), "ok.conf", "the key seeded is not blamed")

	ds, err = object.NewDataStore(cfg)
	require.NoError(t, err)
	assert.True(t, ds.HasKey("ok.conf"))
	b, err := os.ReadFile(filepath.Join(head, "ok.conf"))
	require.NoError(t, err, "installed by the install that seeded it")
	assert.Equal(t, "ok-content", string(b))
	b, err = os.ReadFile(filepath.Join(otherHead, "ok.conf"))
	require.NoError(t, err, "installed by the refresh of the other service the seeding ran")
	assert.Equal(t, "ok-content", string(b))

	// The source of dump.sql shows up: a write of another key refreshes
	// the service, which fails on the key its own install is to seed, and
	// the write is done all the same.
	require.NoError(t, os.WriteFile(missingSrc, []byte("dump"), 0600))
	err = ds.AddKey("another", []byte("y"))
	warnings, err := object.ConsumersInstallWarnings(err)
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "web fs#1: resolve cfg/web key dump.sql")
	ds, err = object.NewDataStore(cfg)
	require.NoError(t, err)
	assert.True(t, ds.HasKey("another"))
	assert.False(t, ds.HasKey("dump.sql"), "a refresh seeds no key")

	// The install of the service seeds it.
	require.NoError(t, installer.Do(context.Background()))
	ds, err = object.NewDataStore(cfg)
	require.NoError(t, err)
	assert.True(t, ds.HasKey("dump.sql"))
}
