package poolshare

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/pool"
	"github.com/opensvc/om3/v3/util/key"
)

// poolConfig is a pool configuration holding the keywords given.
type poolConfig map[string]string

func (c poolConfig) GetString(k key.T) string                { return c[k.Option] }
func (c poolConfig) Eval(k key.T) (any, error)               { return c[k.Option], nil }
func (c poolConfig) GetInt(key.T) int                        { return 0 }
func (c poolConfig) GetStringAs(k key.T, _ string) string    { return c[k.Option] }
func (c poolConfig) GetStringStrict(k key.T) (string, error) { return c[k.Option], nil }
func (c poolConfig) GetStrings(key.T) []string               { return nil }
func (c poolConfig) GetBool(key.T) bool                      { return false }
func (c poolConfig) GetSize(key.T) *int64                    { return nil }
func (c poolConfig) HasSectionString(string) bool            { return true }

func newShare(t *testing.T) *T {
	t.Helper()
	p := New()
	p.SetName("nfs")
	p.SetConfig(poolConfig{"path": "/srv/share"})
	return p
}

// A volume shared by the nodes is a directory of the share, the same on
// every node, and one not shared a directory per node, as v2 laid them out.
func TestTranslate(t *testing.T) {
	p := newShare(t)
	l, err := p.Translate("v1", 0, true)
	require.NoError(t, err)
	assert.Equal(t, []string{"fs#0.type=flag", "fs#1.type=directory", "fs#1.path=/srv/share/v1"}, l)

	l, err = p.Translate("v1", 0, false)
	require.NoError(t, err)
	assert.Equal(t, []string{"fs#0.type=flag", "fs#1.type=directory", "fs#1.path=/srv/share/v1.{nodename}"}, l)
}

// A block volume is a loop file of the share.
func TestBlkTranslate(t *testing.T) {
	p := newShare(t)
	l, err := p.BlkTranslate("v1", 1024, true)
	require.NoError(t, err)
	assert.Equal(t, []string{"disk#0.type=loop", "disk#0.file=/srv/share/v1.img", "disk#0.size={DEFAULT.size}"}, l)
}

// The pool has the capabilities of v2, and not the quota one: it bounds no
// volume, so a namespace claiming it is refused its volumes.
func TestCapabilities(t *testing.T) {
	p := newShare(t)
	assert.Equal(t, "/srv/share", p.Head())
	for _, c := range []pool.Capability{pool.CapROX, pool.CapRWX, pool.CapROO, pool.CapRWO, pool.CapBlk, pool.CapFile, pool.CapShared} {
		assert.True(t, pool.HasCapability(p, c), c)
	}
	assert.False(t, pool.HasCapability(p, pool.CapQuota))
}
