package exe_test

import (
	"testing"

	"github.com/opensvc/om3/v3/util/exe"
	"github.com/opensvc/testhelper"
	"github.com/stretchr/testify/assert"
)

func TestFindExe(t *testing.T) {
	t.Run("returns no exe path when no exe found", func(t *testing.T) {
		td := t.TempDir()
		assert.Equal(t, exe.FindExe(td+"/*"), []string{})
	})

	t.Run("returns non empty path list", func(t *testing.T) {
		td := t.TempDir()
		_, tfCleanup1 := testhelper.TempFileExec(t, td)
		defer tfCleanup1()
		_, tfCleanup2 := testhelper.TempFileExec(t, td)
		defer tfCleanup2()
		_, tfCleanup3 := testhelper.TempFile(t, td)
		defer tfCleanup3()

		for _, pattern := range []string{"", "/*", "/**"} {
			t.Run("with dir"+pattern, func(t *testing.T) {
				assert.Equal(t, len(exe.FindExe(td+pattern)), 2)
			})
		}
	})

	t.Run("result contains exec file", func(t *testing.T) {
		td := t.TempDir()
		tf, tfCleanup := testhelper.TempFileExec(t, td)
		defer tfCleanup()

		for _, pattern := range []string{"", "/*", "/**"} {
			t.Run("with dir"+pattern, func(t *testing.T) {
				assert.Equal(t, exe.FindExe(td+pattern), []string{tf})
			})
		}
	})
}
