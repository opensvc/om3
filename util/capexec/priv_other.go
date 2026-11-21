//go:build !linux

package capexec

import (
	"errors"
)

// errPrivUnsupported is the error of a privilege restriction asked for on an
// operating system om exec can not apply it on: refused, so the command is
// never run believed restricted.
var errPrivUnsupported = errors.New("not supported on this operating system")

func (t T) dropBoundingSet() error {
	if t.CapabilityBoundingSet == nil || *t.CapabilityBoundingSet == "" {
		return nil
	}
	return errors.Join(errors.New("capability bounding set"), errPrivUnsupported)
}

func (t T) setNoNewPrivileges() error {
	if t.NoNewPrivileges == nil || !*t.NoNewPrivileges {
		return nil
	}
	return errors.Join(errors.New("no_new_privs"), errPrivUnsupported)
}
