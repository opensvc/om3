//go:build linux

package storetar

import "golang.org/x/sys/unix"

// localFree returns the space left to a user in the filesystem of dir.
func localFree(dir string) (int64, bool, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return 0, false, err
	}
	return int64(st.Bavail) * st.Bsize, true, nil
}
