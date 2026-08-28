package om

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/commoncmd"
)

// walkCommands calls f on cmd and on every command below it.
func walkCommands(cmd *cobra.Command, f func(*cobra.Command)) {
	f(cmd)
	for _, sub := range cmd.Commands() {
		walkCommands(sub, f)
	}
}

// visibleGroupCommands returns the names of the commands of cmd the usage
// template prints under the group.
func visibleGroupCommands(cmd *cobra.Command, groupID string) []string {
	var l []string
	for _, sub := range cmd.Commands() {
		if sub.GroupID != groupID {
			continue
		}
		if sub.IsAvailableCommand() || sub.Name() == "help" {
			l = append(l, sub.Name())
		}
	}
	return l
}

// A command declaring a group no command of it belongs to used to print the
// group title with nothing under it: "om svc resource --help" had an empty
// "Subsystems:" section.
func TestHelpHasNoEmptyGroupSection(t *testing.T) {
	walkCommands(root, func(cmd *cobra.Command) {
		groups := cmd.Groups()
		if len(groups) == 0 {
			return
		}
		usage := cmd.UsageString()
		for _, group := range groups {
			if len(visibleGroupCommands(cmd, group.ID)) > 0 {
				continue
			}
			assert.NotContainsf(t, usage, group.Title,
				"%q prints the empty %q section", cmd.CommandPath(), group.Title)
		}
	})
}

// The group sections must not lose a command on the way either.
func TestHelpListsEveryVisibleCommand(t *testing.T) {
	walkCommands(root, func(cmd *cobra.Command) {
		if !cmd.HasAvailableSubCommands() {
			return
		}
		usage := cmd.UsageString()
		for _, sub := range cmd.Commands() {
			if !sub.IsAvailableCommand() {
				continue
			}
			assert.Containsf(t, usage, "  "+sub.Name()+" ",
				"%q does not list %q", cmd.CommandPath(), sub.Name())
		}
	})
}

// A command holding subcommands is a subsystem, and belongs to that group
// when its parent sorts its commands in groups. Left ungrouped, it lands in
// the "Additional Commands" section, below the empty group title it should
// have filled.
func TestSubsystemCommandsAreGrouped(t *testing.T) {
	for _, kind := range []string{"svc", "vol", "all"} {
		cmd, _, err := root.Find([]string{kind, "resource", "info"})
		if !assert.NoErrorf(t, err, "om %s resource info", kind) {
			continue
		}
		assert.Equalf(t, "info", cmd.Name(), "om %s resource info", kind)
		assert.Equalf(t, commoncmd.GroupIDSubsystems, cmd.GroupID,
			"om %s resource info is not in the subsystems group", kind)
	}
}

// TestHelpSectionsAreNotEmpty guards the section titles the template prints
// outside of the groups.
func TestHelpSectionsAreNotEmpty(t *testing.T) {
	walkCommands(root, func(cmd *cobra.Command) {
		usage := cmd.UsageString()
		for _, section := range []string{"Additional Commands:", "Available Commands:", "Arguments:", "Flags:"} {
			i := strings.Index(usage, section)
			if i < 0 {
				continue
			}
			rest := strings.TrimLeft(usage[i+len(section):], "\n")
			assert.NotEmptyf(t, strings.TrimSpace(strings.SplitN(rest, "\n", 2)[0]),
				"%q prints the empty %q section", cmd.CommandPath(), section)
		}
	})
}
