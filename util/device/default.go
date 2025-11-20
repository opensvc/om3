//go:build !linux

package device

import "errors"

var ErrNotApplicable = errors.New("not applicable")

func (t T) Holders() (L, error) {
	return nil, ErrNotApplicable
}

func (t T) IsMultipath() (bool, error) {
	return false, ErrNotApplicable
}

func (t T) IsReadOnly() (bool, error) {
	return false, ErrNotApplicable
}

func (t T) IsReadWrite() (bool, error) {
	return false, ErrNotApplicable
}

func (t T) IsReservable() (bool, error) {
	return false, ErrNotApplicable
}

func (t T) IsSCSI() (bool, error) {
	return false, ErrNotApplicable
}

func (t T) Model() (string, error) {
	return "", ErrNotApplicable
}

func (t T) Remove() error {
	return ErrNotApplicable
}

func (t T) SetReadOnly() error {
	return ErrNotApplicable
}

func (t T) SetReadWrite() error {
	return ErrNotApplicable
}

func (t T) SlaveHosts() ([]string, error) {
	return []string{}, ErrNotApplicable
}

func (t T) Slaves() (l L, err error) {
	err = ErrNotApplicable
	return
}

func (t T) Vendor() (string, error) {
	return "", ErrNotApplicable
}

func (t T) WWID() (string, error) {
	return "", nil
}

func (t T) PromoteRW() error {
	return nil
}
