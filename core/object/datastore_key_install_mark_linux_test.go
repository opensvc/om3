package object

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/volfile"
)

// A file an install of a volume writes is marked with its datastore and key,
// an upload mark it had going, and a file found up to date is marked too, as
// one installed before marks were.
func TestInstallKeyToMarksTheFile(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := SetClusterConfig()
	require.NoError(t, err)

	head := t.TempDir()
	probe := filepath.Join(head, ".probe")
	require.NoError(t, os.WriteFile(probe, nil, 0o600))
	if err := unix.Setxattr(probe, "user.probe", []byte("1"), 0); err != nil {
		t.Skipf("no user extended attributes here: %s", err)
	}
	require.NoError(t, os.Remove(probe))

	p := naming.Path{Name: "web", Kind: naming.KindCfg, Namespace: naming.NsRoot}
	o, err := NewDataStore(p)
	require.NoError(t, err)
	require.NoError(t, o.AddKey("app.conf", []byte("v1")))
	dst := filepath.Join(head, "app.conf")
	install := func() {
		_, err := o.InstallKeyTo(KVInstall{
			ToHead:        head,
			ToPath:        dst,
			FromPattern:   "app.conf",
			FromStore:     p,
			AccessControl: KVInstallAccessControl{Perm: 0644, MakedirPerm: 0755},
		})
		require.NoError(t, err)
	}
	mark := func() (volfile.InstallMark, bool) {
		buf := make([]byte, 1024)
		n, err := unix.Lgetxattr(dst, volfile.InstallAttr, buf)
		if err != nil {
			return volfile.InstallMark{}, false
		}
		var m volfile.InstallMark
		require.NoError(t, json.Unmarshal(buf[:n], &m))
		return m, true
	}

	install()
	m, ok := mark()
	require.True(t, ok)
	assert.Equal(t, "cfg/web", m.From, "the path of the datastore")
	assert.Equal(t, "app.conf", m.Key)
	first := m.At

	install()
	m, _ = mark()
	assert.True(t, first.Equal(m.At), "up to date, the mark is kept")

	// Found up to date with no mark, as installed before marks were, and
	// uploaded over meanwhile.
	require.NoError(t, unix.Lremovexattr(dst, volfile.InstallAttr))
	require.NoError(t, unix.Lsetxattr(dst, volfile.UploadAttr, []byte(`{"by":"alice"}`), 0))
	install()
	_, ok = mark()
	assert.True(t, ok, "marked once up to date")
	_, err = unix.Lgetxattr(dst, volfile.UploadAttr, make([]byte, 64))
	assert.Error(t, err, "no longer an upload")
}
