// Package peercred says who is at the other end of a unix socket connection:
// the credentials the kernel recorded when the peer connected, which the
// peer can not choose, and what the process holding them is.
package peercred

import "errors"

type (
	// T is the peer of a unix socket connection, as the kernel knows it.
	T struct {
		// UID, GID and PID are the credentials of the peer at connect
		// time, as SO_PEERCRED reports them in the namespaces of the
		// reader.
		UID uint32
		GID uint32
		PID int32

		// Pinned says the process was read through a pidfd the kernel
		// handed with the connection (SO_PEERPIDFD, Linux 6.5 and
		// later), and still ran once read: Cgroup and MountNS are the
		// ones of the peer, never of a process reusing its pid. Without
		// it, they are left empty.
		Pinned bool

		// Cgroup is the path of the peer in the systemd hierarchy, the
		// "0::" line of cgroup v2 or the "name=systemd" one of v1.
		Cgroup string

		// ForeignMountNS says the peer runs in another mount namespace than the
		// reader, as a container does.
		ForeignMountNS bool
	}
)

var (
	// ErrNotUnix is returned for a connection that is not a unix socket
	// one.
	ErrNotUnix = errors.New("not a unix socket connection")
)
