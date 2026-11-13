package nodeaction

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/actionrouter"
)

// An action submitted to nodes with no remote function says the command has
// no remote form, before asking the api anything: "om node scsi prkey
// --node" answered "RemoteFunc is nil".
func TestDoRemoteWithoutRemoteFunc(t *testing.T) {
	a := New(
		WithRemoteNodes("node1"),
		WithLocalFunc(func() (any, error) { return nil, nil }),
	)
	err := a.DoRemote()
	require.ErrorIs(t, err, ErrNoRemoteFunc)
	assert.NotContains(t, err.Error(), "RemoteFunc")
	assert.ErrorIs(t, a.Do(), ErrNoRemoteFunc, "the node selector routes the action to the nodes")
}

// The ids of the execs started are the default output of an action submitted
// to nodes, unless the command chooses its own, an empty one included.
func TestDefaultOutput(t *testing.T) {
	assert.Contains(t, New(WithRemoteNodes("node1")).DefaultOutput, "SESSION_ID")
	assert.Equal(t, "", New(WithRemoteNodes("node1"), WithDefaultOutput("")).DefaultOutput)
	assert.Equal(t, "tab=A:a", New(WithRemoteNodes("node1"), WithDefaultOutput("tab=A:a")).DefaultOutput)
}

// The results of several nodes render each under its node, in the order of
// the nodes, and a failed node, said on stderr already, renders nothing.
func TestHumanResults(t *testing.T) {
	result := func(node, s string, err error) actionrouter.Result {
		return actionrouter.Result{
			Nodename:      node,
			Error:         err,
			HumanRenderer: func() string { return s },
		}
	}
	assert.Equal(t, "one\n", humanResults([]actionrouter.Result{result("n1", "one\n", nil)}))
	s := humanResults([]actionrouter.Result{
		result("n2", "two\n", nil),
		result("n3", "three\n", errors.New("failed")),
		result("n1", "one\n", nil),
	})
	assert.Regexp(t, `(?s)n1.*\none\n.*n2.*\ntwo\n$`, s)
	assert.NotContains(t, s, "three")
}
