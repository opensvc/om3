package object

import (
	"fmt"
	"os"

	"opensvc.com/opensvc/util/editor"
	"opensvc.com/opensvc/util/file"
)

type OptsEditKey struct {
	Key string `flag:"key"`
}

func (t Keystore) EditKey(opts OptsEditKey) (err error) {
	var (
		refSum []byte
		f      *os.File
	)
	if f, err = t.temporaryKeyFile(opts.Key); err != nil {
		return
	}
	defer os.Remove(f.Name())
	f.Close()
	if refSum, err = file.MD5(f.Name()); err != nil {
		return
	}
	if err = editor.Edit(f.Name()); err != nil {
		return
	}
	if file.HaveSameMD5(refSum, f.Name()) {
		fmt.Println("unchanged")
	} else {
		var b []byte
		if b, err = os.ReadFile(f.Name()); err != nil {
			return
		}
		if err = t.addKey(opts.Key, b); err != nil {
			return
		}
		return t.Config().Commit()
	}
	return nil
}
