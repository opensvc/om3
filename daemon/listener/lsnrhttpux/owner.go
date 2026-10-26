package lsnrhttpux

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"time"
)

const (
	// ownedDirMode and ownedSockMode are the modes of the directory and
	// the socket of a listener owned by a group: its members connect, no
	// one else reaches the socket.
	ownedDirMode  os.FileMode = 0o750
	ownedSockMode os.FileMode = 0o660

	// groupLookupTimeout bounds the lookup of the owner group, which a
	// name service not answering would hold.
	groupLookupTimeout = 3 * time.Second
)

// lookupGID returns the id of the group named s, or of the id s.
//
// The lookup asks the name service, which may be a directory not answering:
// it is given up after groupLookupTimeout, the lookup left to end on its
// own.
func lookupGID(ctx context.Context, s string) (int, error) {
	if gid, err := strconv.Atoi(s); err == nil {
		return gid, nil
	}
	type result struct {
		g   *user.Group
		err error
	}
	c := make(chan result, 1)
	go func() {
		g, err := user.LookupGroup(s)
		c <- result{g, err}
	}()
	ctx, cancel := context.WithTimeout(ctx, groupLookupTimeout)
	defer cancel()
	select {
	case <-ctx.Done():
		return -1, fmt.Errorf("lookup group %s: %w", s, ctx.Err())
	case r := <-c:
		if r.err != nil {
			return -1, r.err
		}
		return strconv.Atoi(r.g.Gid)
	}
}

// prepareOwnedDir makes the directory of the socket, owned by root and gid,
// before the socket is created in it: no one else reaches the socket, even
// before its own mode is set.
func prepareOwnedDir(addr string, gid int) error {
	dir := filepath.Dir(addr)
	if err := os.MkdirAll(dir, ownedDirMode); err != nil {
		return err
	}
	if err := os.Chown(dir, 0, gid); err != nil {
		return err
	}
	return os.Chmod(dir, ownedDirMode)
}

// ownSocket gives the socket to root and gid, and its members read and
// write it.
func ownSocket(addr string, gid int) error {
	if err := os.Chown(addr, 0, gid); err != nil {
		return err
	}
	return os.Chmod(addr, ownedSockMode)
}

// applyOwnerGroup owns the directory and the socket of the listener by the
// group the owner group function says, when it changed.
func (t *T) applyOwnerGroup(ctx context.Context) error {
	group := t.ownerGroup()
	if group == t.group {
		return nil
	}
	gid, err := lookupGID(ctx, group)
	if err != nil {
		return err
	}
	if err := prepareOwnedDir(t.addr, gid); err != nil {
		return err
	}
	if t.listener != nil {
		if err := ownSocket(t.addr, gid); err != nil {
			return err
		}
	}
	if t.group != "" {
		t.log.Infof("socket owned by the group %s (%d), was %s", group, gid, t.group)
	}
	t.group, t.gid = group, gid
	return nil
}
