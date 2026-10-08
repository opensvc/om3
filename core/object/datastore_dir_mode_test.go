package object

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/file"
)

// A directory an install creates on the way to its key gets the configured
// mode, setgid flag included, which mkdir alone refuses or drops. Installing
// again leaves it as it is.
func TestAnInstallCreatesDirectoriesWithTheirSpecialBits(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := SetClusterConfig()
	require.NoError(t, err)

	head := t.TempDir()
	p := naming.Path{Name: "web", Kind: naming.KindCfg, Namespace: naming.NsRoot}
	o, err := NewDataStore(p)
	require.NoError(t, err)
	require.NoError(t, o.AddKey("index.html", []byte("installed")))
	want := 0o750 | os.ModeSetgid
	install := func() {
		_, err := o.InstallKeyTo(KVInstall{
			ToHead:      head,
			ToPath:      filepath.Join(head, "html") + "/",
			FromPattern: "index.html",
			FromStore:   p,
			AccessControl: KVInstallAccessControl{
				Perm:        0o644,
				DirPerm:     want,
				MakedirPerm: want,
			},
		})
		require.NoError(t, err)
		info, err := os.Stat(filepath.Join(head, "html"))
		require.NoError(t, err)
		assert.Equal(t, want, info.Mode()&file.ModeBits)
	}
	install()
	install()
}
