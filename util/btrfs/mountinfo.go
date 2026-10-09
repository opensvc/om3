package btrfs

import (
	"fmt"
	"strings"
)

// MountOfSubvol returns the mount point of the subvolume at path, or of a
// subvolume below it, from the content of a /proc/self/mountinfo.
//
// A mount of a btrfs subvolume has the path of the subvolume as its root. The
// filesystem it is of is not told apart: a subvolume of another btrfs at the
// same path is taken for this one, which a caller checking a subvolume is not
// mounted before replacing it errs on the safe side with.
func MountOfSubvol(mountinfo []byte, path string, below bool) (string, bool) {
	path = "/" + strings.Trim(path, "/")
	for _, line := range strings.Split(string(mountinfo), "\n") {
		left, right, ok := strings.Cut(line, " - ")
		if !ok {
			continue
		}
		lf := strings.Fields(left)
		rf := strings.Fields(right)
		if len(lf) < 5 || len(rf) < 1 || rf[0] != "btrfs" {
			continue
		}
		root := unescapeMountinfo(lf[3])
		if root == path || (below && strings.HasPrefix(root, path+"/")) {
			return unescapeMountinfo(lf[4]), true
		}
	}
	return "", false
}

// unescapeMountinfo reads the octal escapes of /proc/self/mountinfo, as \040
// for a space.
func unescapeMountinfo(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			var v int
			if _, err := fmt.Sscanf(s[i+1:i+4], "%03o", &v); err == nil {
				b.WriteByte(byte(v))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
