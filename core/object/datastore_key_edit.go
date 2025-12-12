package object

import (
	"fmt"
	"os"

	"github.com/opensvc/om3/v3/util/editor"
	"github.com/opensvc/om3/v3/util/file"
)

func (t *dataStore) EditKey(keyName string) (err error) {
	var (
		refSum []byte
		f      *os.File
	)
	if f, err = t.temporaryKeyFile(keyName); err != nil {
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
		if err = t.addKey(keyName, b); err != nil {
			return
		}
		return t.Config().Commit()
	}
	return nil
}
