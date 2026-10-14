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

// The file a key installed is removed with the key, as what the line
// installs: a file of the head no line installed stays, and so does the file
// of a key a failed install could not say anything of. A line taken away
// from the install keyword takes its files with it.
func TestTheFileOfAKeyGoneIsRemoved(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)

	p := naming.Path{Name: "gw", Kind: naming.KindSvc, Namespace: naming.NsRoot}
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte("[DEFAULT]\nnodes = "+hostname.Hostname()+"\n"), 0600))
	o, err := object.NewCore(p)
	require.NoError(t, err)
	cfg := naming.Path{Name: "gw", Kind: naming.KindCfg, Namespace: naming.NsRoot}
	ds, err := object.NewDataStore(cfg)
	require.NoError(t, err)
	require.NoError(t, ds.AddKey("conf.d/a.cfg", []byte("a")))
	require.NoError(t, ds.AddKey("conf.d/b.cfg", []byte("b")))
	require.NoError(t, ds.AddKey("haproxy.cfg", []byte("head")))

	head := t.TempDir()
	lines := []string{
		"/conf.d/", "from", "./cfg/gw", "key", "conf.d/*",
		"/haproxy.cfg", "from", "./cfg/gw", "key", "haproxy.cfg",
	}
	recv := &DataRecv{to: headReceiver{o: o, head: head}, Install: lines}
	install := func() []string {
		_, err := recv.install(context.Background())
		require.NoError(t, err)
		v, err := ReadInstalled(filepath.Join(InstalledDir(p), "volume#1"))
		if err != nil {
			return nil
		}
		return v.Files
	}

	install()
	require.FileExists(t, filepath.Join(head, "conf.d/a.cfg"))
	require.FileExists(t, filepath.Join(head, "conf.d/b.cfg"))
	require.NoError(t, os.WriteFile(filepath.Join(head, "conf.d/hand.cfg"), []byte("h"), 0644))

	ds, err = object.NewDataStore(cfg)
	require.NoError(t, err)
	require.NoError(t, ds.RemoveKey("conf.d/b.cfg"))
	assert.Contains(t, install(), "/conf.d/b.cfg", "the removal is a change the install reports")
	assert.NoFileExists(t, filepath.Join(head, "conf.d/b.cfg"))
	assert.FileExists(t, filepath.Join(head, "conf.d/a.cfg"))
	assert.FileExists(t, filepath.Join(head, "conf.d/hand.cfg"), "a file no line installed")

	require.NoError(t, ds.RemoveKey("conf.d/a.cfg"))
	install()
	assert.NoFileExists(t, filepath.Join(head, "conf.d/a.cfg"), "the last key of the pattern gone")
	assert.FileExists(t, filepath.Join(head, "conf.d/hand.cfg"))

	recv.Install = lines[:5]
	install()
	assert.NoFileExists(t, filepath.Join(head, "haproxy.cfg"), "a line taken away from the install keyword")
}

// Two lines installing the same file: the one whose key is gone leaves the
// file to the other.
func TestAFileAnotherLineHoldsIsKept(t *testing.T) {
	m := installFiles{
		"cfg/a:x:/f": {"/f", "/g"},
		"cfg/b:y:/f": {"/f"},
	}
	assert.True(t, m.heldByOther("cfg/a:x:/f", "/f"))
	assert.False(t, m.heldByOther("cfg/a:x:/f", "/g"))
	assert.False(t, m.heldByOther("cfg/b:y:/f", "/h"))
}

// A file whose removal failed stays recorded, and is removed by a later run:
// a volume read-only for a while, here a directory the confined tree refuses
// to go through, does not leave it forever.
func TestAFileNotRemovedIsRemovedLater(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)

	p := naming.Path{Name: "gw", Kind: naming.KindSvc, Namespace: naming.NsRoot}
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte("[DEFAULT]\nnodes = "+hostname.Hostname()+"\n"), 0600))
	o, err := object.NewCore(p)
	require.NoError(t, err)
	cfg := naming.Path{Name: "gw", Kind: naming.KindCfg, Namespace: naming.NsRoot}
	ds, err := object.NewDataStore(cfg)
	require.NoError(t, err)
	require.NoError(t, ds.AddKey("conf.d/a.cfg", []byte("a")))
	require.NoError(t, ds.AddKey("conf.d/b.cfg", []byte("b")))

	head := t.TempDir()
	recv := &DataRecv{to: headReceiver{o: o, head: head}, Install: []string{"/conf.d/", "from", "./cfg/gw", "key", "conf.d/*"}}
	install := func() {
		_, err := recv.install(context.Background())
		require.NoError(t, err)
	}
	install()
	require.FileExists(t, filepath.Join(head, "conf.d/b.cfg"))

	// conf.d is out of reach for a while: a symlink the tree refuses.
	dir := filepath.Join(head, "conf.d")
	aside := filepath.Join(t.TempDir(), "conf.d")
	require.NoError(t, os.Rename(dir, aside))
	require.NoError(t, os.Symlink(aside, dir))

	ds, err = object.NewDataStore(cfg)
	require.NoError(t, err)
	require.NoError(t, ds.RemoveKey("conf.d/b.cfg"))
	recv.Install = []string{"/srv/", "from", "./cfg/gw", "key", "conf.d/a.cfg"}
	install()
	assert.FileExists(t, filepath.Join(aside, "b.cfg"), "not removed through the symlink")

	require.NoError(t, os.Remove(dir))
	require.NoError(t, os.Rename(aside, dir))
	install()
	assert.NoFileExists(t, filepath.Join(head, "conf.d/b.cfg"), "removed by a later run")
}

// A file written in place of the one a key installed is left when the key
// goes: it is not the file the line installed anymore.
func TestAFileWrittenInPlaceIsLeft(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)

	p := naming.Path{Name: "gw", Kind: naming.KindSvc, Namespace: naming.NsRoot}
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte("[DEFAULT]\nnodes = "+hostname.Hostname()+"\n"), 0600))
	o, err := object.NewCore(p)
	require.NoError(t, err)
	cfg := naming.Path{Name: "gw", Kind: naming.KindCfg, Namespace: naming.NsRoot}
	ds, err := object.NewDataStore(cfg)
	require.NoError(t, err)
	require.NoError(t, ds.AddKey("conf.d/a.cfg", []byte("a")))
	require.NoError(t, ds.AddKey("conf.d/b.cfg", []byte("b")))

	head := t.TempDir()
	recv := &DataRecv{to: headReceiver{o: o, head: head}, Install: []string{"/conf.d/", "from", "./cfg/gw", "key", "conf.d/*"}}
	_, err = recv.install(context.Background())
	require.NoError(t, err)

	b := filepath.Join(head, "conf.d/b.cfg")
	require.NoError(t, os.WriteFile(b, []byte("written by hand"), 0644))
	ds, err = object.NewDataStore(cfg)
	require.NoError(t, err)
	require.NoError(t, ds.RemoveKey("conf.d/b.cfg"))
	_, err = recv.install(context.Background())
	require.NoError(t, err)
	assert.FileExists(t, b, "not the file installed")
	assert.FileExists(t, filepath.Join(head, "conf.d/a.cfg"))
}
