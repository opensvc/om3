package commoncmd

import (
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

// WalkCommands calls f on cmd and on every command below it.
func WalkCommands(cmd *cobra.Command, f func(*cobra.Command)) {
	f(cmd)
	for _, sub := range cmd.Commands() {
		WalkCommands(sub, f)
	}
}

// GroupCommands returns the commands of cmd the usage template prints under
// the group.
func GroupCommands(cmd *cobra.Command, groupID string) []*cobra.Command {
	var l []*cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.GroupID != groupID {
			continue
		}
		if sub.IsAvailableCommand() || sub.Name() == "help" {
			l = append(l, sub)
		}
	}
	return l
}

// CommandNames returns the name of cmd and its aliases, deduplicated: a
// command naming itself in its own alias list is one name, not two.
func CommandNames(cmd *cobra.Command) []string {
	names := append([]string{cmd.Name()}, cmd.Aliases...)
	slices.Sort(names)
	return slices.Compact(names)
}

// HasGroup is true when cmd declares the section.
func HasGroup(cmd *cobra.Command, groupID string) bool {
	return slices.ContainsFunc(cmd.Groups(), func(g *cobra.Group) bool {
		return g.ID == groupID
	})
}

// SectionBody returns what the usage of cmd prints under the section title,
// up to the next empty line, and whether the section is there at all.
func SectionBody(cmd *cobra.Command, title string) (string, bool) {
	usage := cmd.UsageString()
	i := strings.Index(usage, title)
	if i < 0 {
		return "", false
	}
	rest := strings.TrimLeft(usage[i+len(title):], "\n")
	body, _, _ := strings.Cut(rest, "\n\n")
	return body, true
}
