package objectaction

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The ids of the execs started are the default output of an action submitted
// to nodes, unless the command chooses its own, an empty one included: the
// remote function of a compliance run answers the run.
func TestDefaultOutput(t *testing.T) {
	assert.Contains(t, New(WithRemoteNodes("node1")).DefaultOutput, "SESSION_ID")
	assert.Equal(t, "", New(WithRemoteNodes("node1"), WithDefaultOutput("")).DefaultOutput)
}
