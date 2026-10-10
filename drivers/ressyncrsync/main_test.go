package ressyncrsync

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsOneFileSystem(t *testing.T) {
	assert.True(t, isOneFileSystem([]string{"-HAXpogDtrlvx", "--stats"}))
	assert.True(t, isOneFileSystem([]string{"-a", "-x"}))
	assert.True(t, isOneFileSystem([]string{"-a", "--one-file-system"}))
	assert.False(t, isOneFileSystem([]string{"-HAXpogDtrlv", "--stats", "--exclude=x", "-e", "ssh -x"}))
}
