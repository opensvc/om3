package daemonapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The size configured already is not written again, and a size is written
// in the form a resize records the size reached in, not in bytes.
func TestResizeTargetOp(t *testing.T) {
	const mi = 1024 * 1024
	op, write := resizeTargetOp(160*mi, 160*mi, true)
	assert.False(t, write, "the size configured")
	assert.Empty(t, op)

	op, write = resizeTargetOp(200*mi, 160*mi, true)
	assert.True(t, write)
	assert.Equal(t, "size=200mi", op)

	op, write = resizeTargetOp(2048*mi, 0, false)
	assert.True(t, write, "no configured size to compare to")
	assert.Equal(t, "size=2gi", op)

	op, _ = resizeTargetOp(160*mi+512, 160*mi, true)
	assert.Equal(t, "size=167772672", op, "a size of no whole unit is exact, in bytes")
}
