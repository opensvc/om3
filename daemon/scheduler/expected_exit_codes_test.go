package scheduler

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/xerrors"
)

// The scheduled stats push is run accepting the exit code of a push with
// nothing to collect, and the scheduled compliance run the one of modules all
// not applicable to the node, which are no failures of the job. Other jobs
// accept 0 only.
func TestExpectedExitCodes(t *testing.T) {
	assert.Equal(t, []int{xerrors.ExitCodeNothingToCollect}, expectedExitCodes("pushstats"))
	assert.Equal(t, []int{2}, expectedExitCodes("compliance_auto"))
	assert.Empty(t, expectedExitCodes("pushasset"))
}
