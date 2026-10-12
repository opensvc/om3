package rescontainerocibase

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// An image starting with a '-' would be read as an option of the engine run
// command: the resource refuses it, whoever wrote it.
func TestAnImageStartingWithADashIsRefused(t *testing.T) {
	r := &BT{Image: "--privileged"}
	assert.Error(t, r.Configure())
}
