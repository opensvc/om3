package ressharenfs

import (
	"path/filepath"

	"github.com/opensvc/om3/v3/core/resource"
)

// Holds says the exported path.
func (t *T) Holds(src resource.HoldSource) []resource.Hold {
	path, ok := src.Get("path")
	if !ok || path == "" {
		return nil
	}
	return resource.HoldsOf(resource.HoldExport, filepath.Clean(path))
}
