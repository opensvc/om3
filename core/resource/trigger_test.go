package resource

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/trigger"
)

// A trigger runs within the timeout of its action: one that hangs, like an
// apt waiting on a connection the mirror closed, ends the action at its
// deadline with the commands it started, rather than holding it for ever.
func TestAHangingTriggerEndsAtTheActionDeadline(t *testing.T) {
	marker := fmt.Sprintf("omtest-trigger-%d", os.Getpid())
	r := &T{PostProvision: fmt.Sprintf("/bin/bash -c '(exec -a %s sleep 300) & wait'", marker)}
	require.NoError(t, r.SetRID("container#1"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	go func() {
		// The child the trigger started is up while the trigger hangs.
		time.Sleep(500 * time.Millisecond)
		if exec.Command("pgrep", "-f", marker).Run() != nil {
			t.Errorf("the trigger child %s is not running", marker)
		}
	}()
	begin := time.Now()
	err := r.Trigger(ctx, trigger.NoBlock, trigger.Post, trigger.Provision)
	assert.Error(t, err)
	assert.Less(t, time.Since(begin), 10*time.Second)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if exec.Command("pgrep", "-f", marker).Run() != nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = exec.Command("pkill", "-f", marker).Run()
	require.Fail(t, "a command the trigger started outlived it")
}
