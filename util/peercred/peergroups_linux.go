//go:build linux && !386

package peercred

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

// peerGroups returns the supplementary groups of the peer of the socket fd,
// as SO_PEERGROUPS reports them.
func peerGroups(fd int) ([]uint32, error) {
	buf := make([]uint32, 64)
	for {
		n := uint32(len(buf) * 4)
		_, _, errno := unix.Syscall6(unix.SYS_GETSOCKOPT, uintptr(fd), unix.SOL_SOCKET, unix.SO_PEERGROUPS, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)), 0)
		switch errno {
		case 0:
			return buf[:n/4], nil
		case unix.ERANGE:
			// n is the size needed.
			buf = make([]uint32, n/4+1)
		default:
			return nil, errno
		}
	}
}
