package object

import (
	"errors"
	iofs "io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/confined"
	"github.com/opensvc/om3/v3/util/plog"
)

// An install says whether the content of the file changed: a file created
// or rewritten is a change, the same content installed again is not, so
// what reads the file is told to reload for a change only.
func TestInstallKeyToReportsContentChanges(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := SetClusterConfig()
	require.NoError(t, err)

	head := t.TempDir()
	p := naming.Path{Name: "web", Kind: naming.KindCfg, Namespace: naming.NsRoot}
	o, err := NewDataStore(p)
	require.NoError(t, err)
	require.NoError(t, o.AddKey("app.conf", []byte("v1")))
	install := func() bool {
		changed, err := o.InstallKeyTo(KVInstall{
			ToHead:        head,
			ToPath:        filepath.Join(head, "app.conf"),
			FromPattern:   "app.conf",
			FromStore:     p,
			AccessControl: KVInstallAccessControl{Perm: 0644, MakedirPerm: 0755},
		})
		require.NoError(t, err)
		return changed
	}

	assert.True(t, install(), "a file created")
	assert.False(t, install(), "the same content installed again")

	// The store configuration is rewritten with a later time, as by a
	// change of another key, and the content of this one stays the same.
	later := time.Now().Add(time.Minute)
	require.NoError(t, os.Chtimes(p.ConfigFile(), later, later))
	o, err = NewDataStore(p)
	require.NoError(t, err)
	assert.False(t, install(), "the same content of a store changed otherwise")

	require.NoError(t, o.ChangeKey("app.conf", []byte("v2")))
	assert.True(t, install(), "a file rewritten")
	b, err := os.ReadFile(filepath.Join(head, "app.conf"))
	require.NoError(t, err)
	assert.Equal(t, "v2", string(b))
}

// A rendered file is compared to what its key renders to at each install, the
// rendering reading more than the key: a change of what it reads is a change
// of the file, though the store is the same.
func TestInstallKeyToRendersAtEachInstall(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := SetClusterConfig()
	require.NoError(t, err)

	head := t.TempDir()
	p := naming.Path{Name: "web", Kind: naming.KindCfg, Namespace: naming.NsRoot}
	o, err := NewDataStore(p)
	require.NoError(t, err)
	require.NoError(t, o.AddKey("app.conf", []byte("peer=")))
	peer := "n2"
	install := func() bool {
		changed, err := o.InstallKeyTo(KVInstall{
			ToHead:        head,
			ToPath:        filepath.Join(head, "app.conf"),
			FromPattern:   "app.conf",
			FromStore:     p,
			AccessControl: KVInstallAccessControl{Perm: 0644, MakedirPerm: 0755},
			Render: func(_ string, b []byte) ([]byte, error) {
				return append(b, peer...), nil
			},
		})
		require.NoError(t, err)
		return changed
	}
	assert.True(t, install(), "a file created")
	assert.False(t, install(), "the same rendering installed again")
	peer = "n3"
	assert.True(t, install(), "a rendering changed, the store the same")
	b, err := os.ReadFile(filepath.Join(head, "app.conf"))
	require.NoError(t, err)
	assert.Equal(t, "peer=n3", string(b))
}

// failingWriteFS is the node, but for its writes, which fail.
type failingWriteFS struct{ confined.FS }

func (failingWriteFS) WriteFile(string, []byte, iofs.FileMode) error {
	return errors.New("no space left on device")
}

// A write that failed is no change of the file: what reads it has nothing
// to read again, and an install record naming it would tell otherwise.
func TestAFailedWriteIsNoChange(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := SetClusterConfig()
	require.NoError(t, err)

	p := naming.Path{Name: "web", Kind: naming.KindCfg, Namespace: naming.NsRoot}
	o, err := NewDataStore(p)
	require.NoError(t, err)
	ds := &o.(*cfg).dataStore

	head := t.TempDir()
	dst := filepath.Join(head, "app.conf")
	opt := KVInstall{
		ToHead:        head,
		ToPath:        dst,
		AccessControl: KVInstallAccessControl{Perm: 0644},
		ToLog:         plog.NewLogger(zerolog.Nop()),
		fs:            failingWriteFS{confined.Host()},
	}
	changed, err := ds.writeKey(vKey{Key: "app.conf"}, []byte("v1"), opt)
	assert.Error(t, err)
	assert.False(t, changed, "a file the write failed to create")

	require.NoError(t, os.WriteFile(dst, []byte("v0"), 0644))
	changed, err = ds.writeKey(vKey{Key: "app.conf"}, []byte("v1"), opt)
	assert.Error(t, err)
	assert.False(t, changed, "a file the write failed to rewrite")
}

// A write that failed past the removal of what was at its destination, a
// directory or a link, leaves the destination changed: what reads it is told,
// though no file was written there.
func TestAFailedWritePastARemovalIsAChange(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := SetClusterConfig()
	require.NoError(t, err)

	p := naming.Path{Name: "web", Kind: naming.KindCfg, Namespace: naming.NsRoot}
	o, err := NewDataStore(p)
	require.NoError(t, err)
	require.NoError(t, o.AddKey("app.conf", []byte("v1")))
	ds := &o.(*cfg).dataStore

	for name, setup := range map[string]func(dst string){
		"a directory": func(dst string) { require.NoError(t, os.Mkdir(dst, 0755)) },
		"a link":      func(dst string) { require.NoError(t, os.Symlink("/etc/hostname", dst)) },
	} {
		t.Run(name, func(t *testing.T) {
			head := t.TempDir()
			dst := filepath.Join(head, "app.conf")
			setup(dst)
			var wrote []string
			opt := KVInstall{
				ToHead:        head,
				ToPath:        dst,
				AccessControl: KVInstallAccessControl{Perm: 0644, MakedirPerm: 0755},
				ToLog:         plog.NewLogger(zerolog.Nop()),
				Wrote:         func(p string) { wrote = append(wrote, p) },
				fs:            failingWriteFS{confined.Host()},
			}
			changed, err := ds.installFileKey(vKey{Key: "app.conf"}, opt)
			assert.Error(t, err)
			assert.True(t, changed, "the destination removed is a change")
			assert.Equal(t, []string{"/app.conf"}, wrote, "named once")
			assert.NoFileExists(t, dst)
		})
	}
}
