//go:build !windows && !solaris

package lock

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// tryExclusive takes a flock(2) lock on f without waiting, and says whether
// it holds it.
func tryExclusive(f *os.File) (bool, error) {
	err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, unix.EWOULDBLOCK):
		return false, nil
	default:
		return false, err
	}
}

// releaseExclusive has nothing to do: the flock(2) lock is released with the
// file, closed after.
func releaseExclusive(_ *os.File) {}
