package tui

import (
	"regexp"

	"github.com/rivo/tview"
)

// resetRE matches a reset followed by codes turning attributes or colors off,
// as "\x1b[0;22m", which the color package ends a bold text with.
var resetRE = regexp.MustCompile(`\x1b\[0(?:;(?:2[1-9]|39|49|5[45]))+m`)

// translateANSI returns the tview tags of the ANSI escape sequences of s, as
// tview.TranslateANSI does.
//
// tview reads a reset only alone, "\x1b[0m": in "\x1b[0;22m" it skips the
// reset, and the attribute turned off was the only one set, so it writes no
// tag, and the bold of a text runs into the next one, as the first key of a
// section after its bold header. The resets followed by codes turning things
// off are written alone, which they amount to.
func translateANSI(s string) string {
	return tview.TranslateANSI(resetRE.ReplaceAllString(s, "\x1b[0m"))
}
