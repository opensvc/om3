package object

import (
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/util/file"
)

// LockFile is the path of the file to use as an action lock.
func (t *Base) frozenFile() string {
	return filepath.Join(t.varDir(), "frozen")
}

func (t *Base) Freeze() error {
	p := t.frozenFile()
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
	f.Close()
	log.Info().Msg("now frozen")
	return nil
}

func (t *Base) Unfreeze() error {
	p := t.frozenFile()
	if !file.Exists(p) {
		return nil
	}
	err := os.Remove(p)
	if err != nil {
		return err
	}
	log.Info().Msg("now unfrozen")
	return nil
}
