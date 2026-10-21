package resfsdir

import (
	"path/filepath"

	"github.com/opensvc/om3/v3/core/resource"
)

// Holds says the directory, as a mount point is: another object mounting on
// it, or managing it, would take it. A directory in a volume, written
// volume#<n>:/<path> or <vol name>/<path>, is held by the volume.
func (t *T) Holds(src resource.HoldSource) []resource.Hold {
	p, ok := src.Get("path")
	if !ok || !filepath.IsAbs(p) {
		return nil
	}
	return resource.HoldsOf(resource.HoldMountPoint, filepath.Clean(p))
}
