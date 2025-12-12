package freeze

import (
	"os"
	"path/filepath"
	"time"

	"github.com/opensvc/om3/v3/util/file"
)

func Freeze(p string) error {
	if file.Exists(p) {
		return nil
	}
	d := filepath.Dir(p)
	if !file.Exists(d) {
		if err := os.MkdirAll(d, os.ModePerm); err != nil {
			return err
		}
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

func Unfreeze(p string) error {
	if !file.Exists(p) {
		return nil
	}
	if err := os.Remove(p); err != nil {
		return err
	}
	dir := filepath.Dir(p)
	fd, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer fd.Close()
	return fd.Sync()
}

func Frozen(p string) time.Time {
	fi, err := os.Stat(p)
	if err != nil {
		return time.Time{}
	}
	return fi.ModTime()
}
