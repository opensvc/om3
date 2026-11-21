package capexec

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/opensvc/om3/v3/util/capability"
)

// lastCap returns the number of the last capability the running kernel
// knows, from /proc/sys/kernel/cap_last_cap, or the last one this build
// knows when the file can not be read.
func lastCap() int {
	if b, err := os.ReadFile("/proc/sys/kernel/cap_last_cap"); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil {
			return n
		}
	}
	return unix.CAP_LAST_CAP
}

// dropBoundingSet drops from the capability bounding set of the process
// the capabilities the bounding set asked for does not keep, and from its
// inheritable set too: a capability left there would be passed to the
// command by the execve whatever the bounding set. The ambient
// capabilities not in the inheritable set are dropped by the kernel with
// it.
//
// Every capability the kernel knows is considered, the ones this build
// does not name included: a list of capabilities kept drops them, a list
// of capabilities dropped keeps them.
func (t T) dropBoundingSet() error {
	if t.CapabilityBoundingSet == nil || *t.CapabilityBoundingSet == "" {
		return nil
	}
	bs, err := capability.ParseBoundingSet(*t.CapabilityBoundingSet)
	if err != nil {
		return fmt.Errorf("capability bounding set: %w", err)
	}
	last := lastCap()
	for n := 0; n <= last; n++ {
		if bs.Keeps(n) {
			continue
		}
		if err := unix.Prctl(unix.PR_CAPBSET_DROP, uintptr(n), 0, 0, 0); err != nil {
			if errors.Is(err, unix.EINVAL) {
				// A capability the kernel does not know.
				continue
			}
			return fmt.Errorf("drop %s from the capability bounding set: %w", capability.Name(n), err)
		}
	}
	return t.maskInheritable(bs, last)
}

// maskInheritable removes from the inheritable set of the process the
// capabilities bs does not keep.
func (t T) maskInheritable(bs capability.BoundingSet, last int) error {
	hdr := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	var data [2]unix.CapUserData
	if err := unix.Capget(&hdr, &data[0]); err != nil {
		return fmt.Errorf("read the capabilities: %w", err)
	}
	changed := false
	for n := 0; n <= last && n < 64; n++ {
		if bs.Keeps(n) {
			continue
		}
		word, bit := n/32, uint32(1)<<(n%32)
		if data[word].Inheritable&bit != 0 {
			data[word].Inheritable &^= bit
			changed = true
		}
	}
	if !changed {
		return nil
	}
	if err := unix.Capset(&hdr, &data[0]); err != nil {
		return fmt.Errorf("drop the inheritable capabilities: %w", err)
	}
	return nil
}

// setNoNewPrivileges sets the no_new_privs flag of the process, inherited
// by the command and its children.
func (t T) setNoNewPrivileges() error {
	if t.NoNewPrivileges == nil || !*t.NoNewPrivileges {
		return nil
	}
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("set no_new_privs: %w", err)
	}
	return nil
}
