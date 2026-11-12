//go:build !linux && !darwin

package poolshare

import "fmt"

// checkShared refuses the share on the operating systems om can not tell the
// filesystem type of a path on: a share not mounted there would report the
// usage of the local filesystem holding its mount point, and serve volumes
// the other nodes do not see.
func checkShared(path string) error {
	return fmt.Errorf("the share %s can not be told mounted on this operating system", path)
}
