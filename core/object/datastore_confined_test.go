package object

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/testhelper"
)

// installInHead installs the key of a new cfg to path, in a head the caller
// may have planted links in, and returns the file a link could lead to.
func installInHead(t *testing.T, plant func(head, outside string)) (string, string, error) {
	t.Helper()
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := SetClusterConfig()
	require.NoError(t, err)

	base := t.TempDir()
	head := filepath.Join(base, "head")
	require.NoError(t, os.MkdirAll(filepath.Join(head, "html"), 0755))
	outside := filepath.Join(base, "shadow")
	require.NoError(t, os.WriteFile(outside, []byte("orig"), 0600))
	plant(head, outside)

	p := naming.Path{Name: "web", Kind: naming.KindCfg, Namespace: naming.NsRoot}
	o, err := NewDataStore(p)
	require.NoError(t, err)
	require.NoError(t, o.AddKey("index.html", []byte("installed")))
	err = o.InstallKeyTo(KVInstall{
		ToHead:      head,
		ToPath:      filepath.Join(head, "html", "index.html"),
		FromPattern: "index.html",
		FromStore:   p,
		AccessControl: KVInstallAccessControl{
			Perm:        0644,
			MakedirPerm: 0755,
		},
	})
	return head, outside, err
}

func assertUntouched(t *testing.T, outside string) {
	t.Helper()
	b, err := os.ReadFile(outside)
	require.NoError(t, err)
	assert.Equal(t, "orig", string(b), "the file the link led to is not written")
	info, err := os.Stat(outside)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm(), "nor chmodded")
}

// A link planted where a key is installed is replaced by the file, never
// written through: whoever writes in a volume must not lead the agent to write
// a file of the node.
func TestAnInstallReplacesALinkPlantedAtItsPath(t *testing.T) {
	head, outside, err := installInHead(t, func(head, outside string) {
		require.NoError(t, os.Symlink(outside, filepath.Join(head, "html", "index.html")))
	})
	require.NoError(t, err)
	assertUntouched(t, outside)
	p := filepath.Join(head, "html", "index.html")
	info, err := os.Lstat(p)
	require.NoError(t, err)
	assert.True(t, info.Mode().IsRegular(), "the link is replaced by the file")
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, "installed", string(b))
}

// A link planted in place of a directory on the way is not followed out of
// the head.
func TestAnInstallDoesNotFollowADirectoryLinkOutOfTheHead(t *testing.T) {
	_, outside, _ := installInHead(t, func(head, outside string) {
		require.NoError(t, os.RemoveAll(filepath.Join(head, "html")))
		require.NoError(t, os.Symlink(filepath.Dir(outside), filepath.Join(head, "html")))
	})
	assertUntouched(t, outside)
	assert.NoFileExists(t, filepath.Join(filepath.Dir(outside), "index.html"))
}
