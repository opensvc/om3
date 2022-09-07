package file

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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
