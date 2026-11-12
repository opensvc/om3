//go:build linux

package poolshare

import (
	"fmt"
	"syscall"
)

// sharedFSMagics are the filesystems the nodes mount from the same storage,
// by their statfs magic: the network filesystems, and the cluster ones over
// a shared disk.
var sharedFSMagics = map[int64]string{
	0x6969:     "nfs",
	0x517b:     "smb",
	0xfe534d42: "smb2",
	0xff534d42: "cifs",
	0x00c36400: "ceph",
	0x65735546: "fuse",
	0x47504653: "gpfs",
	0x7461636f: "ocfs2",
	0x01161970: "gfs2",
	0x0bd00bd0: "lustre",
	0x5346414f: "afs",
	0x01021997: "9p",
}

// checkShared returns an error when path is not on a filesystem the nodes
// share: not there, or on a local filesystem, as the mount point of the
// share when the share is not mounted.
func checkShared(path string) error {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return fmt.Errorf("the share %s is not mounted: %w", path, err)
	}
	if _, ok := sharedFSMagics[int64(st.Type)]; !ok {
		return fmt.Errorf("the share %s is not mounted: it is on a local filesystem (statfs type %#x)", path, st.Type)
	}
	return nil
}
