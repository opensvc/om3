package logging

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

func newSessionLogFile(p string) (io.Writer, error) {
	getFile := func() (io.WriteCloser, error) {
		if file, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err != nil {
			return nil, err
		} else {
			return file, nil
		}
	}
	file, err := getFile()
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(p), 0744); err != nil {
			return nil, err
		}
		file, err = getFile()
		if err != nil {
			return nil, err
		}
	}
	return file, nil
}
