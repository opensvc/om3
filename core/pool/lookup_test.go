package pool

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQuotaCause(t *testing.T) {
	assert.Empty(t, quotaCause(true, true, 1024), "bounded, sized and claimed")
	assert.Empty(t, quotaCause(false, false, 0), "a directory holds what it is given")
	assert.Contains(t, quotaCause(true, false, 0), "says none")
	assert.Contains(t, quotaCause(false, true, 1024), "does not bound")
}
