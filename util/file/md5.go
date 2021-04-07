package file

import (
	"crypto/md5"
	"io"
	"os"
)

func MD5(p string) ([]byte, error) {
	var (
		f   *os.File
		err error
	)
	if f, err = os.Open(p); err != nil {
		return nil, err
	}
	defer f.Close()
	h := md5.New()
	if _, err = io.Copy(h, f); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

func HaveSameMD5(opts ...interface{}) bool {
	var ref []byte
	for _, o := range opts {
		var (
			b   []byte
			err error
		)

		switch e := o.(type) {
		case string:
			if b, err = MD5(e); err != nil {
				return false
			}
		case []byte:
			b = e
		default:
			b = nil
		}

		if ref == nil {
			ref = b
		} else if string(ref) != string(b) {
			return false
		}
	}
	return true
}
