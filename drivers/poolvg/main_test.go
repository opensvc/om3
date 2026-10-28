package poolvg

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/pool"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/lvm2"
)

func TestVGInfo_Free(t *testing.T) {
	vgInfo := lvm2.VGInfo{VGFree: "0 "}
	size, err := vgInfo.Free()
	require.Nil(t, err)
	require.Equal(t, int64(0), size)
}

// fakeConfig answers the keywords of the pool section from a map.
type fakeConfig map[string]string

func (c fakeConfig) Eval(k key.T) (any, error)               { return c[k.Option], nil }
func (c fakeConfig) GetInt(k key.T) int                      { return 0 }
func (c fakeConfig) GetString(k key.T) string                { return c[k.Option] }
func (c fakeConfig) GetStringAs(k key.T, _ string) string    { return c[k.Option] }
func (c fakeConfig) GetStringStrict(k key.T) (string, error) { return c[k.Option], nil }
func (c fakeConfig) GetStrings(k key.T) []string             { return nil }
func (c fakeConfig) GetBool(k key.T) bool                    { return false }
func (c fakeConfig) GetSize(k key.T) *int64                  { return nil }
func (c fakeConfig) HasSectionString(s string) bool          { return true }

func newTestPool(conf fakeConfig) *T {
	p := New()
	p.SetName("p1")
	p.SetConfig(conf)
	return p
}

// A pool of thick volumes is the volume group, and takes no snapshot.
func TestThickPool(t *testing.T) {
	p := newTestPool(fakeConfig{"name": "data"})
	assert.Equal(t, "data", p.Head())
	assert.NotContains(t, p.Capabilities(), pool.CapSnap)
	kws, err := p.BlkTranslate("v1", 0, false)
	require.NoError(t, err)
	for _, kw := range kws {
		assert.NotContains(t, kw, "thin_pool")
	}
}

// A pool of thin volumes is its thin pool, named as the volumes charge it,
// snapshots them, and has them made in the thin pool.
func TestThinPool(t *testing.T) {
	p := newTestPool(fakeConfig{"name": "data", "thin_pool": "tp1"})
	assert.Equal(t, "data/tp1", p.Head())
	assert.Contains(t, p.Capabilities(), pool.CapSnap)
	kws, err := p.BlkTranslate("v1", 0, false)
	require.NoError(t, err)
	assert.Contains(t, kws, "disk#0.thin_pool=tp1")
	assert.Contains(t, kws, "disk#0.vg=data")
}
