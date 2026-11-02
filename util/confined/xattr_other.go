//go:build !linux

package confined

import "errors"

var errNoXattr = errors.New("extended attributes are not supported here")

func (t *Tree) SetXattr(string, string, []byte) error   { return errNoXattr }
func (t *Tree) GetXattr(string, string) ([]byte, error) { return nil, errNoXattr }
func (t *Tree) RemoveXattr(string, string) error        { return errNoXattr }
func (host) SetXattr(string, string, []byte) error      { return errNoXattr }
func (host) GetXattr(string, string) ([]byte, error)    { return nil, errNoXattr }
func (host) RemoveXattr(string, string) error           { return errNoXattr }
