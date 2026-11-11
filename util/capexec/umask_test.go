//go:build !windows

package capexec

import (
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The umask asked for is set for the command, 0 included, and passed on the
// om exec command line. An unset one leaves the inherited umask, and a value
// that is no octal umask is refused rather than ignored.
func TestUmask(t *testing.T) {
	saved := syscall.Umask(0o022)
	t.Cleanup(func() { syscall.Umask(saved) })
	current := func() int {
		v := syscall.Umask(0)
		syscall.Umask(v)
		return v
	}
	s := func(v string) *string { return &v }

	require.NoError(t, T{Umask: s("027")}.setUmask())
	assert.Equal(t, 0o027, current())
	require.NoError(t, T{Umask: s("000")}.setUmask())
	assert.Equal(t, 0, current(), "a umask of 0 is set too")
	require.NoError(t, T{}.setUmask())
	assert.Equal(t, 0, current(), "an unset umask leaves the inherited one")

	for _, bad := range []string{"8", "0778", "1000", "u=rwx", "-022"} {
		assert.Errorf(t, T{Umask: s(bad)}.setUmask(), "umask %q", bad)
	}

	assert.Equal(t, []string{"--umask", "022"}, T{Umask: s("022")}.Argv())
	assert.Empty(t, T{}.Argv())
}
