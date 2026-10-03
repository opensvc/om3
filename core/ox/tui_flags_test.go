package ox

import (
	"slices"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"github.com/opensvc/om3/v3/core/tui"
)

// tuiCommand is the command line counterpart of a ":do" action of the TUI.
func tuiCommand(scope, action string) []string {
	switch scope {
	case "object":
		return []string{"svc", action}
	case "instance":
		switch action {
		case "switch", "takeover":
			// The TUI switches to, or takes over on, the instances
			// selected: the object command, with its destination.
			return []string{"svc", action}
		}
		return []string{"svc", "instance", action}
	case "resource":
		// An action on resources is the instance action, with --rid.
		return []string{"svc", "instance", action}
	case "node":
		return []string{"node", action}
	}
	return nil
}

// The booleans of the command line that are no parameter of the api, or that
// the TUI hides: what a command line flag of a ":do" action is when the TUI
// does not offer it. A flag added to a command is either one the api takes,
// which the TUI then offers, or one to list here.
var tuiNotOffered = map[string]string{
	"help":             "command line help",
	"ignore-not-found": "the selection of the TUI names what exists",
	"no-lock":          "not a parameter of the api",
	"quiet":            "command line output",
	"wait":             "command line output: the TUI shows the states as they change",
	"watch":            "command line output: the TUI is the watch",
	"follow":           "command line output: the logs of the action streamed on the terminal",
	"cron":             "hidden by the TUI: the scheduler's",
	"confirm":          "hidden by the TUI: it confirms a run",
}

// A flag the TUI offers on an action is the one the command line takes on
// the same action, and a boolean flag the command line takes is offered by
// the TUI, or known not to be.
func TestTUIDoFlags(t *testing.T) {
	for scope, actions := range tui.DoFlags() {
		for action, offered := range actions {
			path := tuiCommand(scope, action)
			if path == nil {
				t.Errorf("%s %s: no command line counterpart declared", scope, action)
				continue
			}
			cmd, _, err := root.Find(path)
			if err != nil || cmd == root || cmd.Name() != action {
				t.Errorf("%s %s: command %v not found", scope, action, path)
				continue
			}
			name := strings.Join(path, " ")
			for _, flag := range offered {
				f := cmd.Flag(strings.TrimPrefix(flag, "--"))
				switch {
				case f == nil:
					t.Errorf("the TUI offers %s on %s %s, which %s does not take", flag, scope, action, name)
				case f.Hidden:
					t.Errorf("the TUI offers %s on %s %s, which %s hides", flag, scope, action, name)
				case f.Value.Type() != "bool":
					t.Errorf("the TUI offers %s on %s %s as a boolean, and %s takes a %s", flag, scope, action, name, f.Value.Type())
				}
			}
			check := func(f *pflag.Flag) {
				if f.Hidden || f.Value.Type() != "bool" {
					return
				}
				if _, ok := tuiNotOffered[f.Name]; ok {
					return
				}
				if !slices.Contains(offered, "--"+f.Name) {
					t.Errorf("%s takes --%s, which the TUI does not offer on %s %s", name, f.Name, scope, action)
				}
			}
			cmd.Flags().VisitAll(check)
			cmd.InheritedFlags().VisitAll(check)
		}
	}
}
