package omcmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/util/hostname"
)

// The capabilities of a node are read from the node that scanned them, so
// asking for this node's must reach the file and never the daemon: not to
// fetch them, and not to expand the selector naming them either.
//
// isThisNodeOnly is what keeps the daemon out of it, and the answer it gives
// for the empty selector matters as much as the one it gives for the name:
// empty is what a caller building this command otherwise than from a command
// line leaves behind, where the option carries the name of this node.
func TestNodeCapabilitiesListReadsThisNodeWithoutTheDaemon(t *testing.T) {
	localhost := hostname.Hostname()
	for _, tc := range []struct {
		selector string
		expected bool
	}{
		{"", true},
		{localhost, true},

		// Every other selector is a question only the daemon can answer, this
		// node among several included.
		{"*", false},
		{"othernode", false},
		{localhost + ",othernode", false},
	} {
		cmd := CmdNodeCapabilitiesList{NodeSelector: tc.selector}
		assert.Equalf(t, tc.expected, cmd.isThisNodeOnly(), "selector %q", tc.selector)
	}
}
