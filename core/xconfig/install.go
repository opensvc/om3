package xconfig

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/lock"
)

type (
	// Base is the configuration file a write was checked against, as read
	// before the checks.
	//
	// A write is checked, by whoever writes, against the configuration read
	// before it, and lands as a whole file. A file another writer changed
	// in between is replaced by one checked against its predecessor, which
	// puts back what the other write changed, without its checks. A write
	// given a Base lands only if the file is still that Base.
	Base struct {
		data   []byte
		exists bool
	}
)

var (
	// ErrConfigChanged is a write refused because the configuration file
	// is no longer the Base it was checked against.
	ErrConfigChanged = errors.New("the configuration changed while the write was checked")

	// installLockTimeout bounds the wait for the lock of a configuration
	// file, which its holders keep for a rename and a sync.
	installLockTimeout = 10 * time.Second
)

// ReadBase reads the configuration file at p as the Base of a write, before
// the write is checked. A file missing is a Base too: the write lands only if
// nobody created the file meanwhile.
func ReadBase(p string) (Base, error) {
	data, err := os.ReadFile(p)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return Base{}, nil
	case err != nil:
		return Base{}, err
	}
	return Base{data: data, exists: true}, nil
}

// SetBase makes the next write of the configuration land only over base,
// and fail with ErrConfigChanged otherwise.
func (t *T) SetBase(base Base) {
	t.base = &base
}

// installLockPath is the file locked while a configuration file is replaced.
//
// It is kept out of the configuration directory, where the daemon watches
// the files, in the lock directory, at the path the configuration file has
// in the configuration directory. A configuration file out of it is locked
// by a hidden file beside it.
func installLockPath(p string) string {
	if rawconfig.Paths.Lock != "" && rawconfig.Paths.Etc != "" {
		if rel, err := filepath.Rel(rawconfig.Paths.Etc, p); err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.Join(rawconfig.Paths.Lock, "config", rel)
		}
	}
	return filepath.Join(filepath.Dir(p), "."+filepath.Base(p)+".lock")
}

// install replaces the configuration file at dst with the file at src, under
// the lock every writer of dst takes, and only over base when there is one.
//
// The lock makes the comparison with the base and the rename one step: no
// other write lands between the two, whether the daemon, another om process
// or the replication of a peer's file makes it. It is a flock(2) lock, which
// the kernel releases when its holder dies.
func install(src, dst string, base *Base) error {
	ctx, cancel := context.WithTimeout(context.Background(), installLockTimeout)
	defer cancel()
	unlock, err := lock.Exclusive(ctx, installLockPath(dst))
	if err != nil {
		return err
	}
	defer unlock()
	if base != nil {
		current, err := ReadBase(dst)
		if err != nil {
			return err
		}
		if current.exists != base.exists || !bytes.Equal(current.data, base.data) {
			return fmt.Errorf("%w: %s", ErrConfigChanged, dst)
		}
	}
	if err := os.Rename(src, dst); err != nil {
		return err
	}
	return file.Sync(dst)
}

// InstallFile replaces the configuration file at dst with the file at src,
// under the lock every writer of dst takes. It is how a configuration made
// elsewhere, as one fetched from a peer, is installed.
func InstallFile(src, dst string) error {
	return install(src, dst, nil)
}
