package object

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// node.max_parallel takes 0, the number of cpus, or the minimum at least: a
// lower one the daemon does not honor, and would raise.
func TestValidMaxParallel(t *testing.T) {
	for _, s := range []string{"0", "2", "10", " 4 "} {
		assert.NoError(t, validMaxParallel(s), s)
	}
	for _, s := range []string{"1", "-1", "x"} {
		assert.Error(t, validMaxParallel(s), s)
	}
}
