package commoncmd

import (
	"bytes"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// colorized renders src with the colors on, and returns what ColorizeINI
// returned and what it wrote past its return, which is meant to be nothing.
func colorized(t *testing.T, src string) (rendered string, leaked string) {
	t.Helper()
	t.Setenv("NO_COLOR", "")
	savedOutput, savedNoColor := color.Output, color.NoColor
	buf := bytes.NewBuffer(nil)
	color.Output, color.NoColor = buf, false
	t.Cleanup(func() { color.Output, color.NoColor = savedOutput, savedNoColor })
	b := ColorizeINI([]byte(src))
	return string(b), buf.String()
}

// TestColorizeINIWritesNothingOfItsOwn pins that the rendering is the
// returned bytes and nothing else.
//
// It used to set each color with color.Set, which writes the escape sequence
// to the process output as a side effect and returns the color to render
// with. Every colored element therefore left a bare, never reset sequence
// ahead of the whole rendering.
func TestColorizeINIWritesNothingOfItsOwn(t *testing.T) {
	_, leaked := colorized(t, "[fs#1]\n# why this flag\ntype = flag  # inline\nnodes = {clusternodes}\n")
	assert.Empty(t, leaked, "the rendering must be the returned bytes, not a side effect")
}

// TestColorizeINISectionHeaderCarriesNoCommentAttribute pins the symptom the
// leak showed: the italic of a comment was still on when the first line was
// drawn, so the section header of a commented section was rendered italic.
func TestColorizeINISectionHeaderCarriesNoCommentAttribute(t *testing.T) {
	const italic = "\x1b[3m"
	for _, src := range []string{
		"[fs#1]\n# why this flag\ntype = flag\n",
		"# why this section\n[fs#1]\ntype = flag\n",
		"[DEFAULT]\nnodes = *\n\n# why this section\n[fs#1]\ntype = flag\n",
	} {
		rendered, leaked := colorized(t, src)
		require.Empty(t, leaked)

		// The first thing drawn opens with its own attributes, so nothing a
		// later line sets can reach it.
		first, _, _ := bytes.Cut([]byte(rendered), []byte("\n"))
		assert.Truef(t, bytes.HasPrefix(first, []byte("\x1b[")), "the first line must open its own sequence, got %q", first)
		assert.NotContainsf(t, string(first), italic, "the first line of %q must not be italic", src)
	}
}
