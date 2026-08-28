// Package helptest holds the checks the om and ox command trees must both
// pass. It lives outside of commoncmd so the two root commands, which
// commoncmd knows nothing about, can each run them against their own tree.
package helptest

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/commoncmd"
)

// Run runs every command tree check on root.
func Run(t *testing.T, root *cobra.Command) {
	t.Helper()
	t.Run("no empty group section", func(t *testing.T) { NoEmptyGroupSection(t, root) })
	t.Run("no empty section", func(t *testing.T) { NoEmptySection(t, root) })
	t.Run("every visible command listed", func(t *testing.T) { EveryVisibleCommandListed(t, root) })
	t.Run("subsystem commands grouped", func(t *testing.T) { SubsystemCommandsGrouped(t, root) })
	t.Run("no duplicate command name", func(t *testing.T) { NoDuplicateCommandName(t, root) })
	t.Run("groups declared by the parent", func(t *testing.T) { GroupsDeclaredByParent(t, root) })
}

// NoEmptyGroupSection checks that no command prints a section title with
// nothing under it: "om svc resource --help" used to print an empty
// "Subsystems:" section.
func NoEmptyGroupSection(t *testing.T, root *cobra.Command) {
	t.Helper()
	commoncmd.WalkCommands(root, func(cmd *cobra.Command) {
		if len(cmd.Groups()) == 0 {
			return
		}
		usage := cmd.UsageString()
		for _, group := range cmd.Groups() {
			if len(commoncmd.GroupCommands(cmd, group.ID)) > 0 {
				continue
			}
			assert.NotContainsf(t, usage, group.Title,
				"%q prints the empty %q section", cmd.CommandPath(), group.Title)
		}
	})
}

// NoEmptySection checks the section titles the usage template prints outside
// of the groups.
func NoEmptySection(t *testing.T, root *cobra.Command) {
	t.Helper()
	sections := []string{"Additional Commands:", "Available Commands:", "Arguments:", "Flags:"}
	commoncmd.WalkCommands(root, func(cmd *cobra.Command) {
		for _, section := range sections {
			body, ok := commoncmd.SectionBody(cmd, section)
			if !ok {
				continue
			}
			assert.NotEmptyf(t, body,
				"%q prints the empty %q section", cmd.CommandPath(), section)
		}
	})
}

// EveryVisibleCommandListed checks the sections do not lose a command on the
// way.
func EveryVisibleCommandListed(t *testing.T, root *cobra.Command) {
	t.Helper()
	commoncmd.WalkCommands(root, func(cmd *cobra.Command) {
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

// SubsystemCommandsGrouped checks that a command holding subcommands belongs
// to a section of its parent usage, be it the subsystems one or the resource
// groups one. Left ungrouped it lands in the "Additional Commands" section,
// among the verbs of the parent, and below the section title it should have
// filled.
//
// Only the parents offering a subsystems section are checked.
func SubsystemCommandsGrouped(t *testing.T, root *cobra.Command) {
	t.Helper()
	commoncmd.WalkCommands(root, func(cmd *cobra.Command) {
		if !commoncmd.HasGroup(cmd, commoncmd.GroupIDSubsystems) {
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

// NoDuplicateCommandName checks that two commands of a same parent do not
// share a name or an alias: cobra runs the first one and the second is
// unreachable, its help line printed twice.
//
// The hidden commands a visible one shadows are skipped: those are backward
// compatibility spellings, not duplicates.
func NoDuplicateCommandName(t *testing.T, root *cobra.Command) {
	t.Helper()
	commoncmd.WalkCommands(root, func(cmd *cobra.Command) {
		seen := make(map[string]string)
		for _, sub := range cmd.Commands() {
			if !sub.IsAvailableCommand() {
				continue
			}
			for _, name := range commoncmd.CommandNames(sub) {
				assert.Emptyf(t, seen[name],
					"%q calls both %q and %q %q",
					cmd.CommandPath(), seen[name], sub.Name(), name)
				seen[name] = sub.Name()
			}
		}
	})
}

// GroupsDeclaredByParent checks that every command belongs to a section its
// parent declares. Cobra panics on the first Execute otherwise, wherever in
// the tree the offending command sits.
func GroupsDeclaredByParent(t *testing.T, root *cobra.Command) {
	t.Helper()
	commoncmd.WalkCommands(root, func(cmd *cobra.Command) {
		for _, sub := range cmd.Commands() {
			if sub.GroupID == "" {
				continue
			}
			assert.Truef(t, commoncmd.HasGroup(cmd, sub.GroupID),
				"%q belongs to the %q section, which %q does not declare",
				sub.CommandPath(), sub.GroupID, cmd.CommandPath())
		}
	})
}
