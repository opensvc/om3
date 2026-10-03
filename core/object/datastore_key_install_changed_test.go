package object

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/testhelper"
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
