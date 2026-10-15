package flagfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A flag raised to be taken away again is taken away only while it is the
// one raised: a flag raised before, or raised again by another after it was
// lowered, is left alone.
func TestRaise(t *testing.T) {
	p := filepath.Join(t.TempDir(), "var", "stopped")

	raised, err := Raise(p)
	require.NoError(t, err)
	require.NotNil(t, raised)
	assert.FileExists(t, p)
	require.NoError(t, raised.Lower())
	assert.NoFileExists(t, p, "the flag raised is lowered")

	raised, err = Raise(p)
	require.NoError(t, err)
	require.NoError(t, raised.Keep())
	assert.FileExists(t, p, "the flag raised is kept")

	raised, err = Raise(p)
	require.NoError(t, err)
	assert.Nil(t, raised, "a flag raised before is not this call's to lower")

	require.NoError(t, Unset(p))
	raised, err = Raise(p)
	require.NoError(t, err)
	require.NoError(t, os.Remove(p))
	require.NoError(t, Set(p))
	require.NoError(t, raised.Lower())
	assert.FileExists(t, p, "a flag raised again by another is left alone")
}
