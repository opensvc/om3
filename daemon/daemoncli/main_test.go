package daemoncli

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"opensvc.com/opensvc/util/usergroup"
)

func privileged() bool {
	ok, err := usergroup.IsPrivileged()
	if err == nil && ok {
		return true
	}
	return false
}

func TestDaemonStartThenStop(t *testing.T) {
	if !privileged() {
		t.Skip("need root")
	}
	require.False(t, Running())
	go func() {
		require.Nil(t, Start())
	}()
	require.Nil(t, WaitRunning())
	require.True(t, Running())
	require.Nil(t, Stop())
	require.False(t, Running())
}

func TestDaemonReStartThenStop(t *testing.T) {
	if !privileged() {
		t.Skip("need root")
	}
	require.False(t, Running())
	go func() {
		require.Nil(t, ReStart())
	}()
	require.Nil(t, WaitRunning())
	require.True(t, Running())
	require.Nil(t, Stop())
	require.False(t, Running())
}

func TestStop(t *testing.T) {
	if !privileged() {
		t.Skip("need root")
	}
	require.False(t, Running())
	require.Nil(t, Stop())
	require.False(t, Running())
}

func TestDaemonStartThenEventsReadAtLeastOneEvent(t *testing.T) {
	//if !privileged() {
	//	t.Skip("need root")
	//}
	go func() {
		require.Nil(t, Start())
	}()
	require.Nil(t, WaitRunning())

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	// smallest size of event to read
	b := make([]byte, 87)
	go func() {
		require.Nil(t, Events())
	}()
	_, err := r.Read(b)
	require.Nil(t, err)
	os.Stdout = old
	readString := string(bytes.TrimRight(b, "\x00"))
	fmt.Printf("Read: %s\n", readString)

	require.Containsf(t, readString, "demo msg xxx",
		"Expected '%s' in \n%s\n", "demo msg xxx", readString)
}
