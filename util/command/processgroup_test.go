package command_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/command"
)

// A command run in its own process group dies with the processes it started
// when its context is cancelled: a script whose child hangs holding its
// output ends at the deadline, and leaves nothing behind.
func TestAProcessGroupIsKilledAsAWholeOnCancel(t *testing.T) {
	marker := fmt.Sprintf("omtest-pgroup-%d", os.Getpid())
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cmd := command.New(
		command.WithContext(ctx),
		command.WithProcessGroup(),
		command.WithName("/bin/bash"),
		command.WithVarArgs("-c", fmt.Sprintf("(exec -a %s sleep 300) & wait", marker)),
		command.WithBufferedStdout(),
	)
	go func() {
		// The child is up while the command hangs.
		time.Sleep(500 * time.Millisecond)
		if exec.Command("pgrep", "-f", marker).Run() != nil {
			t.Errorf("the child %s is not running", marker)
		}
	}()
	begin := time.Now()
	err := cmd.Run()
	assert.Error(t, err)
	assert.Less(t, time.Since(begin), 10*time.Second, "the command ends at its deadline")

	// The background child is gone with its group.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if exec.Command("pgrep", "-f", marker).Run() != nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = exec.Command("pkill", "-f", marker).Run()
	require.Fail(t, "the background child outlived the cancelled command")
}
