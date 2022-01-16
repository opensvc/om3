package daemoncli

import (
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
