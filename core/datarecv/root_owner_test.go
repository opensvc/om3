package datarecv

import (
	"os"
	"syscall"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/util/plog"
)

// With neither dirperm nor perm set, the user and group keywords give the
// volume root its owner alone: a group set alone changes the group only, the
// user and the mode left as they are, and the status says the root owner
// differs until it is given.
func TestTheRootOwnerIsGivenWithoutAMode(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("chown needs root")
	}
	head := t.TempDir()
	require.NoError(t, os.Chmod(head, 0o700))
	receiver := dirModeReceiver{head: head, log: plog.NewLogger(zerolog.Nop()), status: resource.NewStatusLog()}
	recv := &DataRecv{to: receiver, Group: "4242"}

	recv.statusRootOwner(head)
	assert.NotZero(t, receiver.status.Len(), "the root group differs")

	require.NoError(t, recv.installRootDir("/", head, recv.User, recv.Group))
	info, err := os.Stat(head)
	require.NoError(t, err)
	stat := info.Sys().(*syscall.Stat_t)
	assert.Equal(t, uint32(0), stat.Uid, "the user is not set: left as it is")
	assert.Equal(t, uint32(4242), stat.Gid)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm(), "no mode set: left as it is")

	status := resource.NewStatusLog()
	recv.to = dirModeReceiver{head: head, log: plog.NewLogger(zerolog.Nop()), status: status}
	recv.statusRootOwner(head)
	assert.Zero(t, status.Len(), "the root owner given")
}

// With no user nor group set, the root is left alone.
func TestTheRootIsLeftAloneWithNoOwnerSet(t *testing.T) {
	head := t.TempDir()
	receiver := dirModeReceiver{head: head, log: plog.NewLogger(zerolog.Nop()), status: resource.NewStatusLog()}
	recv := &DataRecv{to: receiver}
	require.NoError(t, recv.installRootDir("/", head, "", ""))
	recv.statusRootOwner(head)
	assert.Zero(t, receiver.status.Len())
}
