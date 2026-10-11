package discover

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/naming"
)

// The path and the rid of an install record are read back from its name, in
// the root namespace and in another.
func TestInstalledFilenameToPathAndRID(t *testing.T) {
	for _, s := range []string{"svc1", "ns1/svc/svc1"} {
		p, err := naming.ParsePath(s)
		require.NoError(t, err)
		path, rid, err := installedFilenameToPathAndRID(filepath.Join(datarecv.InstalledDir(p), "fs#1"))
		require.NoError(t, err)
		assert.Equal(t, p, path)
		assert.Equal(t, "fs#1", rid)
	}
}
