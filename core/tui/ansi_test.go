package tui

import (
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/commoncmd"
)

// The bold of a section header ends with it: the first key of the section is
// not bold.
func TestTranslateANSIEndsTheBoldOfASectionHeader(t *testing.T) {
	noColor := color.NoColor
	color.NoColor = false
	defer func() { color.NoColor = noColor }()
	s := translateANSI(string(commoncmd.ColorizeINI([]byte("[DEFAULT]\nnodes = *\n"))))
	assert.Equal(t, "[yellow::b][DEFAULT][-:-:-]\n[teal:]nodes[-:-:-][gray:] = [-:-:-]*\n", s)
	assert.True(t, resetRE.MatchString("\x1b[0;22;23m"))
	assert.False(t, resetRE.MatchString("\x1b[0;31m"), "a reset followed by a color is not a plain reset")
	assert.False(t, resetRE.MatchString("\x1b[0;1m"), "nor one followed by bold")
}
