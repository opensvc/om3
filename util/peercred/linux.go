//go:build linux

package peercred

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// FromConn returns the peer of the unix socket connection c.
//
// The cgroup and the mount namespace of the process are read too, and a peer
// whose process can not be read is not returned: a peer whose namespace is
// unknown could be a container taken for a process of the node, so it is
// refused rather than returned half known. The process is read through the
// pidfd of the connection where the kernel gives one, and by its pid
// otherwise.
func FromConn(c net.Conn) (*T, error) {
	uc, ok := c.(*net.UnixConn)
	if !ok {
		return nil, ErrNotUnix
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return nil, err
	}
	var (
		cred     *unix.Ucred
		credErr  error
		pidfd    = -1
		pidfdErr error
	)
	if err := raw.Control(func(fd uintptr) {
		cred, credErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
		pidfd, pidfdErr = unix.GetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_PEERPIDFD)
	}); err != nil {
		return nil, err
	}
	if credErr != nil {
		return nil, credErr
	}
	t := &T{UID: cred.Uid, GID: cred.Gid, PID: cred.Pid}
	if pidfdErr != nil || pidfd < 0 {
		pidfd = -1
	} else {
		defer unix.Close(pidfd)
	}
	if err := t.readProcess(pidfd); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnknownProcess, err)
	}
	return t, nil
}

// readProcess reads the cgroup and the mount namespace of the peer from its
// pid, then makes sure, through the pidfd when there is one, that the process
// read is still the peer: a pid reused meanwhile would have shown another
// process. Without a pidfd the process read is the one holding the pid then,
// which the peer still is unless it ended and its pid was given again in the
// meantime.
func (t *T) readProcess(pidfd int) error {
	head := "/proc/" + strconv.Itoa(int(t.PID))
	b, err := os.ReadFile(head + "/cgroup")
	if err != nil {
		return err
	}
	mnt, err := os.Readlink(head + "/ns/mnt")
	if err != nil {
		return err
	}
	self, err := os.Readlink("/proc/self/ns/mnt")
	if err != nil {
		return err
	}
	if pidfd >= 0 {
		if err := unix.PidfdSendSignal(pidfd, 0, nil, 0); err != nil {
			return fmt.Errorf("the peer ended: %w", err)
		}
		t.Pinned = true
	}
	t.Cgroup = systemdCgroup(b)
	t.ForeignMountNS = mnt != self
	return nil
}

// systemdCgroup returns the path of the systemd hierarchy in a
// /proc/<pid>/cgroup content: the "name=systemd" line on cgroup v1, the
// "0::" one on cgroup v2.
func systemdCgroup(b []byte) string {
	var unified string
	for _, line := range bytes.Split(b, []byte("\n")) {
		fields := strings.SplitN(string(line), ":", 3)
		if len(fields) != 3 {
			continue
		}
		switch {
		case fields[1] == "name=systemd":
			return fields[2]
		case fields[0] == "0" && fields[1] == "":
			unified = fields[2]
		}
	}
	return unified
}
