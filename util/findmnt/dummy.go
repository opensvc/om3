// +build !linux

package findmnt

import "errors"

func Has(dev string, mnt string) (bool, error) {
	return false, errors.New("not implemented")
}
