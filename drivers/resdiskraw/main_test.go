package resdiskraw

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsNamed(t *testing.T) {
	for _, s := range []string{"data", "db-redo", "disk#1"} {
		assert.True(t, isNamed(s), s)
	}
	for _, s := range []string{"", "/dev/sdb", "/dev/mapper/*", "sd?", "dm-[0-3]"} {
		assert.False(t, isNamed(s), s)
	}
}
