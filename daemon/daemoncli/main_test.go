package daemoncli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDaemonStartThenStop(t *testing.T) {
	require.False(t, Running())
	go func() {
		require.Nil(t, Start())
	}()
	time.Sleep(100 * time.Millisecond)
	require.Nil(t, WaitRunning())
	require.True(t, Running())
	require.Nil(t, Stop())
	require.False(t, Running())
}

func TestDaemonReStartThenStop(t *testing.T) {
	require.False(t, Running())
	go func() {
		require.Nil(t, ReStart())
	}()
	time.Sleep(100 * time.Millisecond)
	require.Nil(t, WaitRunning())
	require.True(t, Running())
	require.Nil(t, Stop())
	require.False(t, Running())
}

func TestStop(t *testing.T) {
	require.False(t, Running())
	require.Nil(t, Stop())
	require.False(t, Running())
}
