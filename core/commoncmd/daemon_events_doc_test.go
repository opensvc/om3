package commoncmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/daemon/msgbus"
)

func TestWrapWords(t *testing.T) {
	assert.Equal(t, []string{"aaa bbb", "ccc"}, wrapWords("aaa bbb ccc", 7))
	assert.Equal(t, []string{"a", "toolongword", "b"}, wrapWords("a toolongword b", 5))
	assert.Nil(t, wrapWords("  ", 10))
}

// TestRenderEventDocs pins the doc of a kind for a terminal: its name, its
// text wrapped and indented, the keys of its data aligned.
func TestRenderEventDocs(t *testing.T) {
	doc, err := msgbus.GetDoc("NodeNetworkStatusUpdated")
	require.NoError(t, err)
	s := renderEventDocs([]msgbus.Doc{doc})
	assert.Contains(t, s, "NodeNetworkStatusUpdated\n\n  A node set up a network.\n\n  Published")
	assert.Contains(t, s, "    node            string\n    name            string\n    network_status  netstatus.Status\n")
	for _, line := range strings.Split(s, "\n") {
		assert.LessOrEqual(t, len(line), eventDocWidth+2, line)
	}
}
