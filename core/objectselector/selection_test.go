package objectselector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A command needing an object, given none, says so, rather than an empty
// path not found.
func TestMustExpandOfNoSelectorSaysNoObjectIsSelected(t *testing.T) {
	_, err := New("").MustExpand()
	require.ErrorIs(t, err, ErrNoSelector)
	assert.NotContains(t, err.Error(), ": object not found")
}
