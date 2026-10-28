package daemonapi

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/testhelper"
)

// The provisioned state of a resource is what the actions of the node wrote:
// none, as for an instance never provisioned on the node, is not
// provisioned, and a state that can not be read is an error, not a "no".
func TestIsResourceProvisioned(t *testing.T) {
	testhelper.Setup(t)
	p := naming.Path{Namespace: "ns1", Kind: naming.KindVol, Name: "v1"}

	v, err := isResourceProvisioned(p, "disk#2")
	require.NoError(t, err)
	assert.False(t, v, "no state is not provisioned")

	write := func(s string) {
		dir := filepath.Join(p.VarDir(), "disk#2")
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "provisioned"), []byte(s), 0644))
	}
	write("true\n")
	v, err = isResourceProvisioned(p, "disk#2")
	require.NoError(t, err)
	assert.True(t, v)

	write("false\n")
	v, err = isResourceProvisioned(p, "disk#2")
	require.NoError(t, err)
	assert.False(t, v)

	write("garbage")
	_, err = isResourceProvisioned(p, "disk#2")
	assert.Error(t, err, "a state that can not be read keeps the file")
}

// The rid of the request names a directory of the var directory of the
// object: only the id of a disk resource is taken.
func TestIsDiskRID(t *testing.T) {
	for _, s := range []string{"disk#1", "disk#drbd"} {
		assert.True(t, isDiskRID(s), s)
	}
	for _, s := range []string{"", "disk", "fs#1", "../disk#1", "disk#1/../..", "disk#1/x", "DEFAULT", "env"} {
		assert.False(t, isDiskRID(s), s)
	}
}
