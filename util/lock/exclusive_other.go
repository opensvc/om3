//go:build windows || solaris

package lock

import (
	"os"
	"sync"
)

// exclusiveMu stands for the flock(2) lock where there is none: it excludes
// the goroutines of this process, and not the other processes.
var exclusiveMu sync.Mutex

func tryExclusive(_ *os.File) (bool, error) {
	return exclusiveMu.TryLock(), nil
}
