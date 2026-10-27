//go:build linux && 386

package peercred

import "golang.org/x/sys/unix"

// peerGroups reports SO_PEERGROUPS unsupported on 386, where getsockopt is
// not a system call of its own: the groups are read from the process.
func peerGroups(fd int) ([]uint32, error) {
	return nil, unix.ENOPROTOOPT
}
