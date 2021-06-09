// +build linux

package device

import (
	"fmt"
	"os"
	"strings"

	"opensvc.com/opensvc/util/file"
)

func (t T) roFile() string {
	return fmt.Sprintf("/sys/block/%s/ro", t)
}

func (t T) IsReadWrite() (bool, error) {
	if ro, err := t.IsReadOnly(); err != nil {
		return false, err
	} else {
		return !ro, nil
	}
}

func (t T) IsReadOnly() (bool, error) {
	if b, err := file.ReadAll(t.roFile()); err != nil {
		return false, err
	} else {
		return strings.TrimSpace(string(b)) == "1", nil
	}
}

func (t T) PromoteReadWrite() error {
	f, err := os.Create(t.roFile())
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write([]byte("1"))
	return err
}
