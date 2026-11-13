package editor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An editor given with its arguments is split as a shell splits words.
func TestCommandSplitsTheEditorWithItsArguments(t *testing.T) {
	for s, expected := range map[string][]string{
		"vi":                      {"vi"},
		"code --wait":             {"code", "--wait"},
		"emacs -nw":               {"emacs", "-nw"},
		`sed -i "s/a b/c/"`:       {"sed", "-i", "s/a b/c/"},
		`'/opt/my editor/bin/ed'`: {"/opt/my editor/bin/ed"},
	} {
		argv, err := command(s)
		require.NoError(t, err, s)
		assert.Equal(t, expected, argv, s)
	}
	_, err := command("   ")
	assert.Error(t, err)
	_, err = command(`vi "unterminated`)
	assert.Error(t, err)
}
