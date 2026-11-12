//go:build linux

package poolshare

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/opensvc/om3/v3/util/mountinfo"
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
	0x47504653: "gpfs",
	0x7461636f: "ocfs2",
	0x01161970: "gfs2",
	0x0bd00bd0: "lustre",
	0x5346414f: "afs",
	0x01021997: "9p",
}

const fuseMagic = 0x65735546

// sharedFUSETypes are the fuse filesystems the nodes mount from the same
// storage, by the type mountinfo gives them: a fuse filesystem may as well be
// local to the node, its statfs type telling none of what it serves.
var sharedFUSETypes = map[string]bool{
	"fuse.glusterfs": true,
	"fuse.ceph-fuse": true,
}

// checkShared returns an error when path is not on a filesystem the nodes
// share: not there, or on a local filesystem, as the mount point of the
// share when the share is not mounted.
func checkShared(path string) error {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return fmt.Errorf("the share %s is not mounted: %w", path, err)
	}
	if int64(st.Type) == fuseMagic {
		entries, err := mountinfo.Read()
		if err != nil {
			return fmt.Errorf("the share %s is on a fuse filesystem, whose type can not be read: %w", path, err)
		}
		return checkSharedFUSE(path, entries)
	}
	if _, ok := sharedFSMagics[int64(st.Type)]; !ok {
		return fmt.Errorf("the share %s is not mounted: it is on a local filesystem (statfs type %#x)", path, st.Type)
	}
	return nil
}

// checkSharedFUSE returns an error when the fuse mount holding path, the one
// of the deepest mount point holding it, the last made, is not of a fuse
// filesystem the nodes share.
func checkSharedFUSE(path string, entries []mountinfo.Entry) error {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	path = filepath.Clean(path)
	var holder *mountinfo.Entry
	for i, e := range entries {
		target := filepath.Clean(e.Target)
		if path != target && !strings.HasPrefix(path, strings.TrimSuffix(target, "/")+"/") {
			continue
		}
		if holder == nil || len(target) >= len(filepath.Clean(holder.Target)) {
			holder = &entries[i]
		}
	}
	if holder == nil {
		return fmt.Errorf("the share %s is on a fuse filesystem no mount holds", path)
	}
	if !sharedFUSETypes[holder.FSType] {
		return fmt.Errorf("the share %s is on a %s filesystem, not one the nodes share", path, holder.FSType)
	}
	return nil
}
