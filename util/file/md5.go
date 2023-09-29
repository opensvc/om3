package file

import (
	"crypto/md5"
	"io"
	"os"
)

// MD5 returns the []byte format md5 of the content of the file at path p.
func MD5(p string) ([]byte, error) {
	if f, err := os.Open(p); err != nil {
		return nil, err
	} else {
		defer f.Close()
		return MD5Reader(f)
	}
}

func MD5Reader(r io.Reader) ([]byte, error) {
	h := md5.New()
	if _, err := io.Copy(h, r); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

// HaveSameMD5 accepts a variadic list of options. Each option can be
// either a []byte format md5 or a file path. In the latter case, the
// md5 is computed inline to compare with the previous known md5.
//
// HaveSameMD5 returns true if all options refer directly or indirectly
// to the same md5 checksum.
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
