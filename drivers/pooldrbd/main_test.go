//go:build linux

package pooldrbd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/util/key"
)

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

func newTestPool(t *testing.T, conf fakeConfig) *T {
	t.Helper()
	if cluster.ConfigData.IsSet() {
		previous := cluster.ConfigData.Get()
		t.Cleanup(func() { cluster.ConfigData.Set(previous) })
	}
	cluster.ConfigData.Set(&cluster.Config{Name: "cluster1", Nodes: []string{"n1", "n2"}})
	p := New()
	p.SetName("p1")
	p.SetConfig(conf)
	return p
}

// The filesystem of a volume of a drbd pool is one the nodes replicate, so it
// is shared whatever the volume says of shared: with fs#1.shared=false, the
// node provisioning the secondary ran a mkfs on a device it can not write,
// and the provision of a volume with no shared keyword failed there.
func TestTheFilesystemOfADrbdPoolVolumeIsShared(t *testing.T) {
	cases := map[string]struct {
		conf fakeConfig
		want []string
	}{
		"a volume group": {
			conf: fakeConfig{"vg": "data", "fs_type": "xfs"},
			want: []string{"fs#1.type=xfs", "fs#1.dev=disk#2", "fs#1.shared=true"},
		},
		"a zpool": {
			conf: fakeConfig{"zpool": "tank", "fs_type": "ext4"},
			want: []string{"fs#1.dev=disk#2", "fs#1.shared=true"},
		},
		"a loop file": {
			conf: fakeConfig{"path": "/srv/pool", "fs_type": "ext4"},
			want: []string{"fs#1.dev=disk#4", "fs#1.shared=true"},
		},
		"a zfs filesystem": {
			conf: fakeConfig{"vg": "data", "fs_type": "zfs"},
			want: []string{"disk#0.type=zpool", "disk#0.shared=true", "fs#1.type=zfs", "fs#1.shared=true"},
		},
	}
	for name, tc := range cases {
		for _, shared := range []bool{false, true} {
			t.Run(name, func(t *testing.T) {
				p := newTestPool(t, tc.conf)
				kws, err := p.Translate("v1", 1<<30, shared)
				require.NoError(t, err)
				for _, kw := range tc.want {
					assert.Contains(t, kws, kw, "volume shared=%v", shared)
				}
				for _, kw := range kws {
					assert.NotContains(t, kw, "shared=false", "volume shared=%v", shared)
				}
			})
		}
	}
}

// A block volume has no filesystem, and nothing of the pool to share.
func TestABlockVolumeOfADrbdPoolHasNoFilesystem(t *testing.T) {
	p := newTestPool(t, fakeConfig{"vg": "data", "fs_type": "xfs"})
	kws, err := p.BlkTranslate("v1", 1<<30, false)
	require.NoError(t, err)
	for _, kw := range kws {
		assert.NotContains(t, kw, "fs#1.")
		assert.NotContains(t, kw, "shared=")
	}
}
