package freeze

import (
	"os"
	"path/filepath"
	"time"

	"github.com/opensvc/om3/util/file"
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
	return f.Close()
}

func Unfreeze(p string) error {
	if !file.Exists(p) {
		return nil
	}
	return os.Remove(p)
}

func Frozen(p string) time.Time {
	fi, err := os.Stat(p)
	if err != nil {
		return time.Time{}
	}
	return fi.ModTime()
}
