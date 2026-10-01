package omcmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/testhelper"
)

// A configuration file that can not be looked at is not taken for one that
// exists: the create says it can not tell, and why, rather than that the
// object already exists.
func TestObjectExistsSaysWhyItCanNotTell(t *testing.T) {
	testhelper.Setup(t)
	p, err := naming.ParsePath("test/svc/toto")
	require.NoError(t, err)

	exists, err := objectExists(p)
	require.NoError(t, err)
	require.False(t, exists)

	cf := p.ConfigFile()
	require.NoError(t, os.MkdirAll(filepath.Dir(cf), 0o755))

	// A path stat refuses to resolve, as root would see it too.
	require.NoError(t, os.Symlink(cf, cf))
	_, err = objectExists(p)
	require.ErrorContains(t, err, "can not tell whether test/svc/toto exists")
	require.NoError(t, os.Remove(cf))

	require.NoError(t, os.WriteFile(cf, []byte("[DEFAULT]\n"), 0o644))
	exists, err = objectExists(p)
	require.NoError(t, err)
	require.True(t, exists)

	if os.Geteuid() == 0 {
		return
	}
	// A directory this user can not look into.
	dir := filepath.Dir(cf)
	require.NoError(t, os.Chmod(dir, 0o000))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	_, err = objectExists(p)
	require.ErrorContains(t, err, "om creates an object as root")
}
