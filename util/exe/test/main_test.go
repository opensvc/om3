package exe_test

import (
	"github.com/stretchr/testify/assert"
	"opensvc.com/opensvc/test_helper"
	"opensvc.com/opensvc/util/exe"
	"testing"
)

func TestFindExe(t *testing.T) {
	t.Run("returns no exe path when no exe found", func(t *testing.T) {
		td, tdCleanup := test_helper.Tempdir(t)
		defer tdCleanup()
		assert.Equal(t, exe.FindExe(td+"/*"), []string{})
	})

	t.Run("returns non empty path list", func(t *testing.T) {
		td, tdCleanup := test_helper.Tempdir(t)
		defer tdCleanup()
		_, tfCleanup1 := test_helper.TempFileExec(t, td)
		defer tfCleanup1()
		_, tfCleanup2 := test_helper.TempFileExec(t, td)
		defer tfCleanup2()
		_, tfCleanup3 := test_helper.TempFile(t, td)
		defer tfCleanup3()

		for _, pattern := range []string{"", "/*", "/**"} {
			t.Run("with dir"+pattern, func(t *testing.T) {
				assert.Equal(t, len(exe.FindExe(td+pattern)), 2)
			})
		}
	})

	t.Run("result contains exec file", func(t *testing.T) {
		td, tdCleanup := test_helper.Tempdir(t)
		defer tdCleanup()

		tf, tfCleanup := test_helper.TempFileExec(t, td)
		defer tfCleanup()

		for _, pattern := range []string{"", "/*", "/**"} {
			t.Run("with dir"+pattern, func(t *testing.T) {
				assert.Equal(t, exe.FindExe(td+pattern), []string{tf})
			})
		}
	})
}
