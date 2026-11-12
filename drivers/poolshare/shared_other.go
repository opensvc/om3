//go:build !linux

package poolshare

// checkShared does not judge the filesystem of path on the operating systems
// whose statfs says no type.
func checkShared(string) error {
	return nil
}
