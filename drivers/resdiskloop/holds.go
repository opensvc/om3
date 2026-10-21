package resdiskloop

import (
	"path/filepath"

	"github.com/opensvc/om3/v3/core/resource"
)

// Holds says the file the loop device is backed by.
func (t *T) Holds(src resource.HoldSource) []resource.Hold {
	file, ok := src.Get("file")
	if !ok || file == "" {
		return nil
	}
	return resource.HoldsOf(resource.HoldFile, filepath.Clean(file))
}
