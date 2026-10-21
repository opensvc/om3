package resfshost

import (
	"path/filepath"

	"github.com/opensvc/om3/v3/core/resource"
)

// Holds says the mount point the filesystem is mounted on, and the block
// device it is made on, when it names one: a tmpfs, a bind mount or a network
// filesystem holds no device.
func (t *T) Holds(src resource.HoldSource) []resource.Hold {
	l := make([]resource.Hold, 0)
	if mnt, ok := src.Get("mnt"); ok && mnt != "" {
		l = append(l, resource.HoldsOf(resource.HoldMountPoint, filepath.Clean(mnt))...)
	}
	if dev, ok := src.Get("dev"); ok {
		l = append(l, resource.HoldsOf(resource.HoldBlockDevice, resource.DevicePaths(dev)...)...)
	}
	return l
}
