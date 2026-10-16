package object

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/resource"
)

// mountedResource is a resource mounting a path, the rest of Driver left to
// the nil interface it embeds.
type mountedResource struct {
	resource.Driver
	rid     string
	head    string
	standby bool
}

func (t *mountedResource) RID() string      { return t.rid }
func (t *mountedResource) Head() string     { return t.head }
func (t *mountedResource) IsDisabled() bool { return false }
func (t *mountedResource) IsStandby() bool  { return t.standby }

// The resource holding the source of a sync is the one whose mount point
// holds it, the deepest: it is the one a standby would keep mounted for a
// switch to send the sync.
func TestMountHolding(t *testing.T) {
	l := resource.Drivers{
		&mountedResource{rid: "fs#1", head: "/srv/data", standby: true},
		&mountedResource{rid: "fs#2", head: "/srv/data/sub"},
	}
	assert.Equal(t, "fs#2", mountHolding(l, "/srv/data/sub/x/").RID(), "the deepest")
	assert.Equal(t, "fs#1", mountHolding(l, "/srv/data/").RID(), "the mount point itself, a trailing slash or not")
	assert.True(t, mountHolding(l, "/srv/data").IsStandby())
	assert.Nil(t, mountHolding(l, "/srv/database"), "a path sharing a prefix is not held")
	assert.Nil(t, mountHolding(l, "/other"))

	hint := finalSyncHint("sync#2", "/srv/data/sub/", []string{"n2"}, mountHolding(l, "/srv/data/sub/"))
	assert.Contains(t, hint, "sync#2: not sent to n2 before the switch")
	assert.Contains(t, hint, "A standby fs#2")
}
