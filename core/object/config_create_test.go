package object

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/key"
)

// A created configuration leaves behind what the configuration it was copied
// from recorded of itself, and has an id of its own, unless restored: then it
// keeps them, and is given an id only when it holds none.
func TestCreatedConfig(t *testing.T) {
	p, err := naming.ParsePath("test/svc/clone")
	require.NoError(t, err)
	src := []byte("[DEFAULT]\nid = 2c8186c1-db46-4482-a4e5-ab45a8e083d6\nnodes = *\n[disk#1]\ntype = md\nuuid = 7d9a2f0e:4b1c2d3e:5f6a7b8c:9d0e1f2a\nlevel = raid1\n")

	oc, b, err := CreatedConfig(p, src, false)
	require.NoError(t, err)
	id := oc.Config().GetString(key.New("DEFAULT", "id"))
	assert.NotEmpty(t, id)
	assert.NotEqual(t, "2c8186c1-db46-4482-a4e5-ab45a8e083d6", id, "an id of its own")
	assert.Empty(t, oc.Config().GetString(key.New("disk#1", "uuid")), "recorded by the source")
	assert.Equal(t, "raid1", oc.Config().GetString(key.New("disk#1", "level")))
	assert.Contains(t, string(b), "[DEFAULT]")
	assert.Contains(t, string(b), "id = "+id)
	assert.NotContains(t, string(b), "7d9a2f0e")

	oc, _, err = CreatedConfig(p, src, true)
	require.NoError(t, err)
	assert.Equal(t, "2c8186c1-db46-4482-a4e5-ab45a8e083d6", oc.Config().GetString(key.New("DEFAULT", "id")), "restored")
	assert.Equal(t, "7d9a2f0e:4b1c2d3e:5f6a7b8c:9d0e1f2a", oc.Config().GetString(key.New("disk#1", "uuid")))

	oc, _, err = CreatedConfig(p, []byte("[DEFAULT]\nnodes = *\n"), true)
	require.NoError(t, err)
	assert.NotEmpty(t, oc.Config().GetString(key.New("DEFAULT", "id")), "restored without an id")
}

// The keyword operations of a creation apply after the reset: a recorded
// keyword they name is the creator's, and kept.
func TestCreatedConfigSetsApplyAfterTheReset(t *testing.T) {
	p, err := naming.ParsePath("test/svc/new")
	require.NoError(t, err)
	oc, _, err := CreatedConfig(p, nil, false, keyop.ParseOps([]string{"disk#1.type=disk", "disk#1.disk_id=aaaa"})...)
	require.NoError(t, err)
	assert.Equal(t, "aaaa", oc.Config().GetString(key.New("disk#1", "disk_id")))
}
