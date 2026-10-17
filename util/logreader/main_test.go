package logreader

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/streamlog"
)

// A console line marks in braille the node it comes from among a few, and
// says the node name alone among more than eight, the marker being too wide
// to read at a glance.
func TestRenderEventMarksTheNodeAmongAFew(t *testing.T) {
	noColor := color.NoColor
	color.NoColor = true
	defer func() { color.NoColor = noColor }()
	render := func(streamIndex, numStreams int) string {
		var b bytes.Buffer
		e := streamlog.Event{M: map[string]any{"JSON": `{"level":"info","message":"foo"}`}}
		assert.NoError(t, renderEvent(e, "node2", streamIndex, numStreams, "", &b))
		return b.String()
	}
	isBraille := func(r rune) bool { return r >= 0x2800 && r <= 0x28FF }

	s := render(1, 3)
	assert.True(t, strings.ContainsFunc(s, isBraille), "3 streams: %q", s)
	assert.Contains(t, s, "⣸⡀ node2: foo")

	s = render(1, 8)
	assert.True(t, strings.ContainsFunc(s, isBraille), "8 streams: %q", s)

	s = render(1, 9)
	assert.False(t, strings.ContainsFunc(s, isBraille), "9 streams: %q", s)
	assert.Contains(t, s, "node2: foo")
	assert.NotContains(t, s, "  node2", "no blank marker left of the node name: %q", s)
}
