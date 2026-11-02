//go:build !linux

package volfile

import "errors"

func probeXattr(string) error { return errors.New("not linux") }
