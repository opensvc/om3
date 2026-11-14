//go:build !unix

package confined

import "io/fs"

// fileOwner tells no owner where the system has no uid.
func fileOwner(fs.FileInfo) (int, bool) {
	return 0, false
}
