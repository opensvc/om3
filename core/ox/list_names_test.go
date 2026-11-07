package ox

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// A command listing things is named list, and answers to ls too: the name
// advertised is the same everywhere, and both are typed.
func TestAListingIsNamedListAliasedLs(t *testing.T) {
	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		assert.NotEqual(t, "ls", cmd.Name(), "%s: name it list, aliased ls", cmd.CommandPath())
		if cmd.Name() == "list" {
			assert.True(t, cmd.HasAlias("ls"), "%s: alias it ls", cmd.CommandPath())
		}
		for _, c := range cmd.Commands() {
			walk(c)
		}
	}
	walk(root)
}
