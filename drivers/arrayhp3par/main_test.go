package arrayhp3par

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/array"
)

// TestEveryReadingAsksForCSV pins that each command asks for the format the
// parser reads. v2 sets it once per ssh session, and the command shapes this
// driver uses do not open one, so a command that forgot would be parsed as if
// it were csv and read as nothing.
func TestEveryReadingAsksForCSV(t *testing.T) {
	assert.Equal(t, " -csvtable -nohdtot", csvArgs)
}

// TestActionsBuildATree pins that what this driver declares is a tree the
// parser can be built from.
func TestActionsBuildATree(t *testing.T) {
	actions := (&Array{}).Actions()
	require.NotEmpty(t, actions)
	_, err := array.NewCommand(actions, &bytes.Buffer{})
	require.NoError(t, err)

	paths := make(map[string]bool)
	for _, action := range actions {
		require.NotEmptyf(t, action.Short, "action %v has no help", action.Path)
		require.NotNilf(t, action.Run, "action %v does nothing", action.Path)
		key := ""
		for _, word := range action.Path {
			key += " " + word
		}
		paths[key] = true
	}
	for _, want := range []string{" get volumes", " get system", " get nodes", " get cpgs", " get ports", " get version"} {
		assert.Truef(t, paths[want], "no action named%s", want)
	}
}

// TestReportsAreTheV2Sections pins the names the collector reads. v2 declares
// no action for a 3par and only pushes these.
func TestReportsAreTheV2Sections(t *testing.T) {
	keys := make([]string, 0)
	for _, report := range (&Array{}).Reports() {
		require.NotNil(t, report.Get)
		keys = append(keys, report.Key)
	}
	assert.Equal(t, []string{"showvv", "showsys", "shownode", "showcpg", "showport", "showversion"}, keys)
}
