package object

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The strict evaluation refuses a reference evaluating only at run time that
// failed or is empty, and leaves the others to the evaluation.
func TestJudgeRuntimeReference(t *testing.T) {
	assert.NoError(t, judgeRuntimeReference("name", "", nil))
	assert.NoError(t, judgeRuntimeReference("env.x", "", fmt.Errorf("boom")))
	assert.NoError(t, judgeRuntimeReference("volume#1.mnt", "/srv/a", nil))
	assert.ErrorContains(t, judgeRuntimeReference("volume#1.mnt", " ", nil), "{volume#1.mnt} evaluates empty")
	assert.ErrorContains(t, judgeRuntimeReference("disk#1.exposed_devs", "", nil), "{disk#1.exposed_devs} evaluates empty")
	assert.ErrorContains(t, judgeRuntimeReference("disk#1.capacity", "disk#1.capacity", fmt.Errorf("has no size yet")), "{disk#1.capacity}: has no size yet")
}
