package btrfs

import (
	"strings"

	"github.com/opensvc/om3/v3/util/mountinfo"
)

// MountOfSubvol returns the mount point of the subvolume at path, or of a
// subvolume below it, from the content of a /proc/self/mountinfo.
//
// A mount of a btrfs subvolume has the path of the subvolume as its root. The
// filesystem it is of is not told apart: a subvolume of another btrfs at the
// same path is taken for this one, which a caller checking a subvolume is not
// mounted before replacing it errs on the safe side with.
func MountOfSubvol(b []byte, path string, below bool) (string, bool) {
	path = "/" + strings.Trim(path, "/")
	for _, m := range mountinfo.Parse(b) {
		if m.FSType != "btrfs" {
			continue
		}
		if m.Root == path || (below && strings.HasPrefix(m.Root, path+"/")) {
			return m.Target, true
		}
	}
	return "", false
}
