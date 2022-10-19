package file

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIsCharDevice(t *testing.T) {
	v, err := IsCharDevice("/dev/null")
	assert.NoError(t, err)
	assert.True(t, v)
}

func TestIsBlockDevice(t *testing.T) {
	v, err := IsBlockDevice("/dev/null")
	assert.NoError(t, err)
	assert.False(t, v)
}

func TestIsDevice(t *testing.T) {
	v, err := IsDevice("/dev/null")
	assert.NoError(t, err)
	assert.True(t, v)
}

func TestTouch(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), t.Name())
	assert.NoError(t, err)
	defer f.Close()
	p := f.Name()
	now := time.Now()
	err = Touch(p, now)
	assert.NoError(t, err)
	mtime := ModTime(p)
	assert.WithinDuration(t, now, mtime, 0)
}
