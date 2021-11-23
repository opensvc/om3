// +build !solaris

package ulimit

import (
	"golang.org/x/sys/unix"
)

func setNProc(value *int64) error {
	return setSizeRlimit(value, unix.RLIMIT_NPROC)
}

func setRss(value *int64) error {
	return setSizeRlimit(value, unix.RLIMIT_RSS)
}

func setMemLock(value *int64) error {
	return setSizeRlimit(value, unix.RLIMIT_MEMLOCK)
}
