package storeplakar

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/opensvc/om3/v3/core/backup"
)

// Usage says the space a store that is a directory takes, the size of its
// files, which is what plakar info counts as the storage size, without the
// key it derives to say so, and the space left in its filesystem. A store
// not written yet takes none, and the space left is the one of where it will
// be.
//
// A remote location says neither: the packages of plakar, sftp and s3 as
// its own http client, do not tell the size of a store, and none tells the
// space left.
func (t *T) Usage(ctx context.Context) (backup.Usage, error) {
	_, dir, err := t.location()
	if err != nil {
		return backup.Usage{}, fmt.Errorf("store %s: %w", t.Ref(), err)
	}
	if dir == "" {
		return backup.Usage{}, nil
	}
	var u backup.Usage
	at := dir
	if ok, err := storeExists(dir); err != nil {
		return u, fmt.Errorf("store %s: %w", t.Ref(), err)
	} else if ok {
		stored, err := dirSize(ctx, dir)
		if err != nil {
			return u, fmt.Errorf("store %s: %w", t.Ref(), err)
		}
		u.StoredBytes = &stored
	} else if _, err := os.Stat(dir); err != nil {
		at = filepath.Dir(dir)
	}
	free, ok, err := localFree(at)
	if err != nil {
		return u, fmt.Errorf("store %s: %w", t.Ref(), err)
	}
	if ok {
		u.FreeBytes = &free
	}
	return u, nil
}

// dirSize sums the sizes of the files under dir, the links not followed. A
// file plakar removes meanwhile, as a lock, is not counted.
func dirSize(ctx context.Context, dir string) (int64, error) {
	var total int64
	err := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	return total, err
}
