package backup

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A note is recorded as a tag by a store: no comma, no line break, and a
// bounded length.
func TestCheckNote(t *testing.T) {
	assert.NoError(t, CheckNote(""))
	assert.NoError(t, CheckNote("before the upgrade to 2.4"))
	assert.Error(t, CheckNote("before upgrade, keep"))
	assert.Error(t, CheckNote("line\nbreak"))
	assert.Error(t, CheckNote(strings.Repeat("x", noteMax+1)))
}
