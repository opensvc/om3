package restask

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidInstall(t *testing.T) {
	assert.NoError(t, validInstall("fs#1"))
	assert.NoError(t, validInstall("fs#1 volume#data"))
	assert.Error(t, validInstall("container#1"))
	assert.Error(t, validInstall("fs#1 task#1"))
}
