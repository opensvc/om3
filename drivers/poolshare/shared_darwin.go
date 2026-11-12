//go:build darwin

package poolshare

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// sharedFSNames are the filesystems the nodes mount from the same storage,
// by the name statfs gives them. A fuse filesystem is not one: its name tells
// none of what it serves, which may be local to the node.
var sharedFSNames = map[string]bool{
	"nfs":   true,
	"smbfs": true,
	"afpfs": true,
	"cifs":  true,
}

// checkShared returns an error when path is not on a filesystem the nodes
// share: not there, or on a local filesystem, as the mount point of the
// share when the share is not mounted.
func checkShared(path string) error {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return fmt.Errorf("the share %s is not mounted: %w", path, err)
	}
	name := unix.ByteSliceToString(st.Fstypename[:])
	if !sharedFSNames[name] {
		return fmt.Errorf("the share %s is not mounted: it is on a local filesystem (%s)", path, name)
	}
	return nil
}
