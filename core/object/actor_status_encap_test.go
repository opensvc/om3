package object

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// What a command run in a container says ends the error of its failure.
func TestLastLines(t *testing.T) {
	assert.Equal(t, "", lastLines("", 3))
	assert.Equal(t, "Error: c/svc/x already exists", lastLines("\nError: c/svc/x already exists\n\n", 3))
	assert.Equal(t, "b; c; d", lastLines("a\nb\n\nc\nd\n", 3))
}
