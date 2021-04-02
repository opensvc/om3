package exe_test

import (
	"github.com/stretchr/testify/assert"
	"opensvc.com/opensvc/util/exe"
	"os"
	"path/filepath"
	"testing"
)

func TestFindExe(t *testing.T) {
	t.Run("returns no exe path when no exe found", func(t *testing.T) {
		assert.Equal(t, exe.FindExe("."), []string{})
	})

	GOROOT := os.Getenv("GOROOT")
	t.Run("FindExe(GOROOT) returns non empty path list", func(t *testing.T) {
		assert.Greater(t, len(exe.FindExe(GOROOT)), 0)
	})

	t.Run("FindExe(GOROOT) contains GOROOT/bin/go", func(t *testing.T) {
		assert.Contains(t, exe.FindExe(GOROOT), filepath.Join(GOROOT, "bin", "go"))
	})
}
