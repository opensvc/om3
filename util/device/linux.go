// +build linux

package device

import (
	"fmt"
	"os"
	"strings"

	"opensvc.com/opensvc/util/file"
)

func (t T) IsReadWrite() (bool, error) {
	if ro, err := t.IsReadOnly(); err != nil {
		return false, err
	} else {
		return !ro, nil
	}
}

func (t T) IsReadOnly() (bool, error) {
	if b, err := file.ReadAll(t.fileRO()); err != nil {
		return false, err
	} else {
		return strings.TrimSpace(string(b)) == "1", nil
	}
}

func (t T) SetReadWrite() error {
	return t.setRO("0")
}

func (t T) SetReadOnly() error {
	return t.setRO("1")
}

func (t T) fileRO() string {
	return fmt.Sprintf("/sys/block/%s/ro", t)
}

func (t T) setRO(s string) error {
	b := []byte(s)
	f, err := os.Create(t.fileRO())
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(b)
	return err
}
