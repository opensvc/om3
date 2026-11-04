//go:build linux

package restask

import (
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A run is signaled only while it is the process of its run, and the end of
// a process signaled is no error.
func TestSignalRunSignalsTheRunOnly(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	require.NoError(t, cmd.Start())
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	pid := cmd.Process.Pid

	require.NoError(t, signalRun(pid, syscall.SIGTERM, func(int) bool { return false }))
	select {
	case <-done:
		t.Fatal("a process no longer its run was signaled")
	case <-time.After(200 * time.Millisecond):
	}

	require.NoError(t, signalRun(pid, syscall.SIGTERM, func(int) bool { return true }))
	select {
	case err := <-done:
		assert.Error(t, err, "ended by the signal")
	case <-time.After(5 * time.Second):
		t.Fatal("the run was not signaled")
	}
	assert.NoError(t, signalRun(pid, syscall.SIGTERM, func(int) bool { return true }), "a process gone")
}
