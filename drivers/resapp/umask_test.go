//go:build !windows

package resapp

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/util/pg"
)

// The umask of the application is passed on to om exec in octal, 0 included,
// and not at all when the keyword is not set.
func TestToCapsUmask(t *testing.T) {
	m := func(v os.FileMode) *os.FileMode { return &v }
	newT := func() *T {
		r := &T{}
		r.SetPG(&pg.Config{})
		return r
	}
	for umask, want := range map[os.FileMode]string{0o022: "022", 0o007: "007", 0: "000"} {
		r := newT()
		r.Umask = m(umask)
		caps := r.toCaps(execCredential{})
		if assert.NotNil(t, caps.Umask) {
			assert.Equal(t, want, *caps.Umask)
		}
	}
	assert.Nil(t, newT().toCaps(execCredential{}).Umask)
}
