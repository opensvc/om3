package resource

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

type fakeTriggerCommander struct {
	cmd string
	err error
}

func (t fakeTriggerCommander) TriggerCommand(rid, hook string) (string, error) {
	if !strings.HasSuffix(hook, "post_provision") || rid != "volume#1" {
		return "", fmt.Errorf("unexpected trigger %s %s", rid, hook)
	}
	return t.cmd, t.err
}

// A trigger runs the command its object evaluates at the time it runs, not
// the one evaluated as the action reached the resource, before it acted, and
// runs nothing when the object can not evaluate it: a blocking trigger fails
// its action on it.
func TestATriggerRunsTheCommandEvaluatedAsItRuns(t *testing.T) {
	dir := t.TempDir()
	stale, fresh := filepath.Join(dir, "stale"), filepath.Join(dir, "fresh")
	r := &T{PostProvision: "/usr/bin/touch " + stale}
	require.NoError(t, r.SetRID("volume#1"))

	r.object = fakeTriggerCommander{cmd: "/usr/bin/touch " + fresh}
	require.NoError(t, r.Trigger(context.Background(), trigger.NoBlock, trigger.Post, trigger.Provision))
	assert.FileExists(t, fresh)
	assert.NoFileExists(t, stale)

	require.NoError(t, os.Remove(fresh))
	r.object = fakeTriggerCommander{err: errors.New("volume#1.post_provision: reference not available: {volume#1.mnt} evaluates empty")}
	err := r.Trigger(context.Background(), trigger.NoBlock, trigger.Post, trigger.Provision)
	assert.ErrorContains(t, err, "{volume#1.mnt} evaluates empty")
	assert.NoFileExists(t, fresh)
	assert.NoFileExists(t, stale)
}

// A trigger whose command evaluated empty as the action reached the resource,
// a reference not available then, is evaluated again as it runs: it was
// taken for a trigger not set, and skipped.
func TestATriggerEvaluatedEmptyBeforeTheActionRuns(t *testing.T) {
	fresh := filepath.Join(t.TempDir(), "fresh")
	r := &T{}
	require.NoError(t, r.SetRID("volume#1"))
	r.object = fakeTriggerCommander{cmd: "/usr/bin/touch " + fresh}
	require.NoError(t, r.Trigger(context.Background(), trigger.NoBlock, trigger.Post, trigger.Provision))
	assert.FileExists(t, fresh)

	r.object = fakeTriggerCommander{err: errors.New("volume#1.post_provision: reference not available: {volume#1.mnt} evaluates empty")}
	assert.ErrorContains(t, r.Trigger(context.Background(), trigger.Block, trigger.Post, trigger.Provision), "evaluates empty", "a blocking trigger fails")
}
