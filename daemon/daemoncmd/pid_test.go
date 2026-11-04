package daemoncmd

import (
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// The pid of a daemon of a previous boot can be the one of a thread of the
// daemon starting: a thread is not a process running, which /proc also
// shows, with the command line of its process.
func TestAThreadIsNotAProcess(t *testing.T) {
	leader, err := isThreadGroupLeader(os.Getpid())
	require.NoError(t, err)
	assert.True(t, leader, "the process")

	// A thread other than the main one: a goroutine locked to its thread
	// while the main goroutine holds the main thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	tidC := make(chan int)
	release := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		tidC <- unix.Gettid()
		<-release
	}()
	tid := <-tidC
	defer close(release)
	if tid == os.Getpid() {
		t.Skip("no thread other than the main one")
	}
	leader, err = isThreadGroupLeader(tid)
	require.NoError(t, err)
	assert.False(t, leader, "a thread of the process")

	leader, err = isThreadGroupLeader(1 << 30)
	require.NoError(t, err)
	assert.False(t, leader, "an id unknown")
}
