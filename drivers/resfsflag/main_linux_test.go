package resfsflag

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/status"
)

// newTestFlag returns a fs.flag resource of ns1/svc/svc1 whose base
// directory is under a sticky directory every user can write in, as
// /dev/shm.
func newTestFlag(t *testing.T) (*T, string) {
	t.Helper()
	shm := t.TempDir()
	require.NoError(t, os.Chmod(shm, 0o1777))
	saved := tmpBase
	tmpBase = filepath.Join(shm, "opensvc")
	t.Cleanup(func() { tmpBase = saved })
	r := &T{
		T:    resource.T{ResourceID: &resourceid.T{Name: "fs#1"}},
		Path: naming.Path{Namespace: "ns1", Kind: naming.KindSvc, Name: "svc1"},
	}
	return r, shm
}

// A start makes the tree of root 0755, whatever the umask, and the flag a
// regular file of root, up; a stop removes it, down.
func TestStartStop(t *testing.T) {
	r, shm := newTestFlag(t)
	old := syscall.Umask(0)
	defer syscall.Umask(old)
	ctx := context.Background()

	require.Equal(t, status.Down, r.Status(ctx))
	require.NoError(t, r.Start(ctx))
	require.Equal(t, status.Up, r.Status(ctx))
	for _, p := range []string{"opensvc", "opensvc/ns1", "opensvc/ns1/svc", "opensvc/ns1/svc/svc1"} {
		fi, err := os.Stat(filepath.Join(shm, p))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o755), fi.Mode().Perm(), p)
	}
	require.NoError(t, r.Start(ctx), "a start of a flag up")
	require.NoError(t, r.Stop(ctx))
	require.Equal(t, status.Down, r.Status(ctx))
	assert.NoFileExists(t, r.file())
	require.NoError(t, r.Stop(ctx), "a stop of a flag down")
}

// A link planted where the flag goes is not followed by the start, which
// truncated its target, and is not a flag up.
func TestALinkIsNotAFlag(t *testing.T) {
	r, shm := newTestFlag(t)
	ctx := context.Background()
	victim := filepath.Join(t.TempDir(), "victim")
	require.NoError(t, os.WriteFile(victim, []byte("data"), 0o644))
	dir := filepath.Join(shm, "opensvc", "ns1", "svc", "svc1")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.Symlink(victim, filepath.Join(dir, "fs#1.flag")))

	require.Equal(t, status.Down, r.Status(ctx))
	require.ErrorContains(t, r.Start(ctx), "is a link")
	b, err := os.ReadFile(victim)
	require.NoError(t, err)
	assert.Equal(t, "data", string(b), "the target of the link is left as it was")
	require.NoError(t, r.Stop(ctx), "a link is no flag to remove")
	assert.FileExists(t, victim)
}

// A directory of the tree a user made first, or another user's flag file,
// is refused: the start fails, the status is down, and nothing of theirs is
// written, removed, or taken over.
func TestATreeOfAnotherUserIsRefused(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("needs root to give a directory to another user")
	}
	r, shm := newTestFlag(t)
	ctx := context.Background()
	base := filepath.Join(shm, "opensvc")
	require.NoError(t, os.Mkdir(base, 0o777))
	require.NoError(t, os.Chown(base, 65534, 65534))

	require.ErrorContains(t, r.Start(ctx), "not trusted")
	assert.NoDirExists(t, filepath.Join(base, "ns1"))
	require.Equal(t, status.Down, r.Status(ctx))
	require.NoError(t, r.Stop(ctx))
	fi, err := os.Stat(base)
	require.NoError(t, err)
	assert.Equal(t, uint32(65534), fi.Sys().(*syscall.Stat_t).Uid, "never taken over")

	require.NoError(t, os.Chown(base, 0, 0))
	require.NoError(t, os.Chmod(base, 0o755))
	dir := filepath.Join(base, "ns1", "svc", "svc1")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fs#1.flag"), nil, 0o644))
	require.NoError(t, os.Chown(filepath.Join(dir, "fs#1.flag"), 65534, 65534))
	require.Equal(t, status.Down, r.Status(ctx), "a flag of another user is not up")
	require.ErrorContains(t, r.Start(ctx), "owned by uid 65534")
}

// A tree of root others can write in, as one made under a umask of 0, is
// made 0755.
func TestALooseTreeIsTightened(t *testing.T) {
	r, shm := newTestFlag(t)
	base := filepath.Join(shm, "opensvc")
	require.NoError(t, os.Mkdir(base, 0o755))
	require.NoError(t, os.Chmod(base, 0o777))
	require.NoError(t, r.Start(context.Background()))
	fi, err := os.Stat(base)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), fi.Mode().Perm())
}

// The daemon makes the base directory at start, of root 0755.
func TestMakeBaseDir(t *testing.T) {
	_, shm := newTestFlag(t)
	require.NoError(t, MakeBaseDir(t.Logf))
	fi, err := os.Stat(filepath.Join(shm, "opensvc"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), fi.Mode().Perm())
}
