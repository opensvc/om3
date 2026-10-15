package resdiskdisk_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/key"

	_ "github.com/opensvc/om3/v3/drivers/resdiskdisk"
)

// TestDiskIDIsNotCloned pins that a configuration copied to make another
// object leaves the disk ids its provision wrote behind, on every node: the
// clone creates its own disk, where it would otherwise take the one of the
// source, its provision creating none once disk_id is set.
func TestDiskIDIsNotCloned(t *testing.T) {
	p, err := naming.ParsePath("test/svc/clone")
	require.NoError(t, err)
	o, err := object.New(p, object.WithConfigData([]byte(`
[disk#1]
type = disk
pool = freenas
size = 1g
disk_id = 6589cfc00000097484f0728d8b2118a6
disk_id@node1 = 6589cfc00000097484f0728d8b2118a7
`)), object.WithVolatile(true))
	require.NoError(t, err)
	cfg := o.(object.Configurer).Config()
	cfg.UnsetRecorded()
	assert.Empty(t, cfg.GetString(key.New("disk#1", "disk_id")))
	assert.Empty(t, cfg.GetString(key.New("disk#1", "disk_id@node1")))
	assert.Equal(t, "freenas", cfg.GetString(key.New("disk#1", "pool")), "what is not recorded is copied")
}
