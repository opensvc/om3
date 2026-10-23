//go:build !linux

package storetar

// localFree says nothing of the space left out of linux.
func localFree(dir string) (int64, bool, error) {
	return 0, false, nil
}
