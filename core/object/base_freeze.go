package object

import (
	"os"
	"path/filepath"

	"opensvc.com/opensvc/util/file"
)

//
// frozenFile is the path of the file to use as the frozen flag.
// The file mtime is loaded as the frozen key value in the
// instance status dataset.
//
func (t *Base) frozenFile() string {
	return filepath.Join(t.varDir(), "frozen")
}

//
// Freeze creates a persistant flag file that prevents orchestration
// of the object instance.
//
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
	t.log.Info().Msg("now frozen")
	return nil
}

//
// Unfreeze removes the persistant flag file that prevents orchestration
// of the object instance.
//
func (t *Base) Unfreeze() error {
	p := t.frozenFile()
	if !file.Exists(p) {
		return nil
	}
	err := os.Remove(p)
	if err != nil {
		return err
	}
	t.log.Info().Msg("now unfrozen")
	return nil
}

//
// Thaw removes the persistant flag file that prevents orchestration
// of the object instance. Synomym of Unfreeze.
//
func (t *Base) Thaw() error {
	return t.Unfreeze()
}
