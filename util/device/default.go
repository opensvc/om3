//go:build !linux

package device

import "errors"

var ErrNotApplicable = errors.New("not applicable")

func (t T) IsReadWrite() (bool, error) {
	return false, ErrNotApplicable
}

func (t T) IsReadOnly() (bool, error) {
	return false, ErrNotApplicable
}

func (t T) Holders() ([]*T, error) {
	return nil, ErrNotApplicable
}

func (t T) Remove() error {
	return ErrNotApplicable
}

func (t T) SetReadWrite() error {
	return ErrNotApplicable
}

func (t T) SetReadOnly() error {
	return ErrNotApplicable
}

func (t T) WWID() (string, error) {
	return "", nil
}
