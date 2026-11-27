//go:build !linux

package pg

import "errors"

// ApplyProc creates the cgroup, set caps, and add the specified process
func (c Config) ApplyProc(pid int) (bool, error) {
	return false, nil
}

func (c Config) Delete() (bool, error) {
	return false, nil
}

// isIgnored reports whether a keyword caps nothing on this node.
func isIgnored(_ string) bool {
	return false
}

// RemoveTree removes the group and every group under it.
func (c Config) RemoveTree() error {
	return nil
}

// ErrNoSlice, ErrNoSystemd and ErrDelegated are the reasons a group takes no
// network filter, which only a linux node running systemd applies.
var (
	ErrNoSlice   = errors.New("the process group is no systemd slice")
	ErrNoSystemd = errors.New("the node does not run systemd")
	ErrDelegated = errors.New("the process group is delegated to a user, whose systemd instance may not filter the network")
)

// CheckIPFilter says why a network filter can not be set on the group.
func (c Config) CheckIPFilter() error {
	return ErrNoSystemd
}

// SetIPFilter sets the network filter of the group.
func (c Config) SetIPFilter(allow, deny []string) error {
	return ErrNoSystemd
}

// IPFilter returns the network filter of the group.
func (c Config) IPFilter() (allow, deny []string, err error) {
	return nil, nil, ErrNoSystemd
}

// HasIPFilter says a network filter was set on the group.
func (c Config) HasIPFilter() bool {
	return false
}

// Dir returns the directory of the group in the unified hierarchy.
func (c Config) Dir() string {
	return ""
}

// HasMembers says the slice of the group holds a unit.
func (c Config) HasMembers() bool {
	return false
}

// HasProcs says the group holds processes of its own, in no unit.
func (c Config) HasProcs() bool {
	return false
}

// Slice returns the systemd slice the group is.
func (c Config) Slice() string {
	return ""
}

// IsUnified says the node holds the unified cgroup hierarchy alone.
func IsUnified() bool {
	return false
}
