package poolshm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/pool"
	"github.com/opensvc/om3/v3/util/key"
)

// poolConfig is a pool configuration holding the keywords given, the section
// of the pool named shm.
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

var _ pool.Config = poolConfig{}

func translate(t *testing.T, c poolConfig) []string {
	t.Helper()
	p := New()
	p.SetName("shm")
	p.SetConfig(c)
	l, err := p.Translate("data", 1048576, false)
	require.NoError(t, err)
	return l
}

// A volume is sized by reference to DEFAULT.size, the size it is claimed
// with, which is where a resize of the volume records the size it reached:
// sized in mnt_opt, the next mount undid every resize.
func TestAVolumeIsSizedByItsClaim(t *testing.T) {
	l := translate(t, poolConfig{})
	assert.Contains(t, l, "fs#0.size={DEFAULT.size}")
	assert.Contains(t, l, "fs#0.mode=700", "the root is the volume users', not everyone's")
	for _, s := range l {
		assert.NotContains(t, s, "mnt_opt", "nothing else is left to say in mount options")
	}
}

// The mode is the pool's mode keyword, or else the mode= of its mnt_opt, which
// is where a pool configured before the keyword says it. What else mnt_opt
// says is kept, less the size and the mode.
func TestAVolumeTakesTheModeOfItsPool(t *testing.T) {
	l := translate(t, poolConfig{"mode": "750", "mnt_opt": "mode=755,noexec,size=10m"})
	assert.Contains(t, l, "fs#0.mode=750", "the keyword wins")
	assert.Contains(t, l, "fs#0.mnt_opt=noexec")

	l = translate(t, poolConfig{"mnt_opt": "noexec,mode=1777"})
	assert.Contains(t, l, "fs#0.mode=1777", "a mode left in mnt_opt is carried to the keyword")
	assert.Contains(t, l, "fs#0.mnt_opt=noexec")
}
