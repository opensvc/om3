package scheduler

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/xerrors"
)

// The scheduled stats push is run accepting the exit code of a push with
// nothing to collect, which is no failure of the job. Other jobs accept 0
// only.
func TestExpectedExitCodes(t *testing.T) {
	assert.Equal(t, []int{xerrors.ExitCodeNothingToCollect}, expectedExitCodes("pushstats"))
	assert.Empty(t, expectedExitCodes("pushasset"))
}
