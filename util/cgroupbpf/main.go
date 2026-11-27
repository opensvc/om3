// Package cgroupbpf says whether a network filter is attached to a cgroup.
//
// systemd enforces the IPAddressAllow and IPAddressDeny of a unit, and of the
// slices it is in, with two BPF programs attached to the cgroup of the unit,
// one for the packets coming in and one for those going out. Where it can
// not attach them, a kernel without BPF for cgroups or a cgroup systemd does
// not manage, it warns in its journal and runs the unit unfiltered. Asking
// the kernel is the one way to know a filter is in force.
package cgroupbpf

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type (
	// Direction is the way of the packets a program filters.
	Direction int

	// Querier counts the programs in force on a cgroup.
	Querier interface {
		// Effective returns the number of programs filtering the packets
		// of the direction for the processes of the cgroup directory:
		// the ones attached to it, and to the cgroups above it.
		Effective(dir string, direction Direction) (int, error)
	}
)

const (
	Ingress Direction = iota
	Egress
)

// ErrNotAttached is the error of a cgroup no network filter of its own is
// in force on.
var ErrNotAttached = errors.New("no network filter is attached")

// mount is where the unified cgroup hierarchy is mounted, replaced by the
// tests.
var mount = "/sys/fs/cgroup"

func (t Direction) String() string {
	if t == Ingress {
		return "ingress"
	}
	return "egress"
}

// Dir returns the cgroup directory of a process in the unified hierarchy,
// the one of the caller for the pid 0.
func Dir(pid int) (string, error) {
	p := "/proc/self/cgroup"
	if pid > 0 {
		p = fmt.Sprintf("/proc/%d/cgroup", pid)
	}
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	return parseDir(f)
}

// parseDir reads the cgroup of the unified hierarchy from the content of a
// /proc/<pid>/cgroup file: its line starting with "0::".
func parseDir(f *os.File) (string, error) {
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if rel, ok := strings.CutPrefix(scanner.Text(), "0::"); ok {
			return filepath.Join(mount, rel), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("%s names no cgroup of the unified hierarchy", f.Name())
}

// Verify says a network filter is in force on the cgroup directory dir that
// is not in force on the cgroup directory base above it: the programs of
// both directions, attached to dir or between base and dir.
//
// Counting against base tells the filter of a unit from programs something
// else attached higher, to the whole node, which filter nothing for the
// unit in particular.
//
// It returns an error wrapping ErrNotAttached when a direction has no
// program of its own, and the error of the query when the kernel refuses it.
func Verify(q Querier, dir, base string) error {
	for _, direction := range []Direction{Ingress, Egress} {
		n, err := q.Effective(dir, direction)
		if err != nil {
			return fmt.Errorf("query the %s programs of %s: %w", direction, dir, err)
		}
		above, err := q.Effective(base, direction)
		if err != nil {
			return fmt.Errorf("query the %s programs of %s: %w", direction, base, err)
		}
		if n <= above {
			return fmt.Errorf("%w to %s for the %s packets", ErrNotAttached, dir, direction)
		}
	}
	return nil
}
