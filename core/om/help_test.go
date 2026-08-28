package om

import (
	"slices"
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

// A command holding subcommands belongs to a section of its parent help, be
// it the subsystems one or the resource groups one. Left ungrouped it lands in
// the "Additional Commands" section, among the verbs of the parent, and below
// the section title it should have filled.
//
// Only the parents offering a subsystems section are checked: the root command
// sorts none of its object kinds and subsystems yet.
func TestSubsystemCommandsAreGrouped(t *testing.T) {
	walkCommands(root, func(cmd *cobra.Command) {
		if !slices.ContainsFunc(cmd.Groups(), func(g *cobra.Group) bool {
			return g.ID == commoncmd.GroupIDSubsystems
		}) {
			return
		}
		for _, sub := range cmd.Commands() {
			if !sub.IsAvailableCommand() || !sub.HasAvailableSubCommands() {
				continue
			}
			assert.NotEmptyf(t, sub.GroupID,
				"%q holds commands but no section of %q lists it",
				sub.CommandPath(), cmd.CommandPath())
		}
	})
}

// Two commands sharing a name under the same parent: cobra runs the first one
// and the second is unreachable, its help line printed twice. "om daemon" had
// two "run" commands, the running one having been built from the run
// constructor.
func TestNoDuplicateCommandName(t *testing.T) {
	walkCommands(root, func(cmd *cobra.Command) {
		seen := make(map[string]string)
		for _, sub := range cmd.Commands() {
			if !sub.IsAvailableCommand() {
				// a hidden command a visible one shadows is a backward
				// compatibility spelling, not a duplicate
				continue
			}
			names := append([]string{sub.Name()}, sub.Aliases...)
			slices.Sort(names)
			for _, name := range slices.Compact(names) {
				assert.Emptyf(t, seen[name],
					"%q calls both %q and %q %q",
					cmd.CommandPath(), seen[name], sub.Name(), name)
				seen[name] = sub.Name()
			}
		}
	})
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
