package chkmcelog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLines(t *testing.T) {
	log := "old error\nopensvc marker 1\nnew error 1\nnew error 2\n"
	assert.Equal(t, int64(2), newLines([]byte(log), "opensvc marker 1"))
	assert.Equal(t, int64(4), newLines([]byte(log), "opensvc marker 0"), "a marker not found counts every line")
	assert.Equal(t, int64(0), newLines([]byte("opensvc marker 1\n"), "opensvc marker 1"))
	assert.Equal(t, int64(0), newLines([]byte(""), ""))
}
