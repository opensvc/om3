//go:build linux

package peercred

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// IDs returns the uid of the peer of the unix socket connection c and its
// groups, its primary one first, as the kernel recorded them at connect:
// the ids the access to the socket file was checked with then.
//
// The supplementary groups are read with SO_PEERGROUPS, Linux 4.13 and
// later. An older kernel does not record them, and they are read from the
// process of the peer by its pid instead, which a peer ending lets another
// process take.
func IDs(c net.Conn) (uint32, []uint32, error) {
	uc, ok := c.(*net.UnixConn)
	if !ok {
		return 0, nil, ErrNotUnix
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return 0, nil, err
	}
	var (
		cred      *unix.Ucred
		credErr   error
		groups    []uint32
		groupsErr error
	)
	if err := raw.Control(func(fd uintptr) {
		cred, credErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
		if credErr == nil {
			groups, groupsErr = peerGroups(int(fd))
		}
	}); err != nil {
		return 0, nil, err
	}
	if credErr != nil {
		return 0, nil, credErr
	}
	if errors.Is(groupsErr, unix.ENOPROTOOPT) {
		groups, groupsErr = procGroups(cred.Pid)
	}
	if groupsErr != nil {
		return 0, nil, fmt.Errorf("the groups of the peer: %w", groupsErr)
	}
	return cred.Uid, append([]uint32{cred.Gid}, groups...), nil
}

// procGroups reads the supplementary groups of the process pid from the
// "Groups:" line of its status.
func procGroups(pid int32) ([]uint32, error) {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(int(pid)) + "/status")
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(b))
	for scanner.Scan() {
		line, ok := strings.CutPrefix(scanner.Text(), "Groups:")
		if !ok {
			continue
		}
		fields := strings.Fields(line)
		groups := make([]uint32, 0, len(fields))
		for _, field := range fields {
			gid, err := strconv.ParseUint(field, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("groups %q: %w", line, err)
			}
			groups = append(groups, uint32(gid))
		}
		return groups, nil
	}
	return nil, fmt.Errorf("no groups in the status of the process %d", pid)
}
