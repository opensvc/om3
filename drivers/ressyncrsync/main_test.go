package ressyncrsync

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsOneFileSystem(t *testing.T) {
	assert.True(t, isOneFileSystem([]string{"-HAXpogDtrlvx", "--stats"}))
	assert.True(t, isOneFileSystem([]string{"-a", "-x"}))
	assert.True(t, isOneFileSystem([]string{"-a", "--one-file-system"}))
	assert.False(t, isOneFileSystem([]string{"-HAXpogDtrlv", "--stats", "--exclude=x", "-e", "ssh -x"}))
}

// TestWillPrepareSyncOnlyWithSnap pins that only a resource copying a
// snapshot takes it before the transfers of the action, and not with
// delay_snap, which has it take it just before its own transfer.
func TestWillPrepareSyncOnlyWithSnap(t *testing.T) {
	ctx := context.Background()
	assert.False(t, (&T{}).WillPrepareSync(ctx), "no snap")
	assert.False(t, (&T{Snap: true, DelaySnap: true}).WillPrepareSync(ctx), "delay_snap")
}
