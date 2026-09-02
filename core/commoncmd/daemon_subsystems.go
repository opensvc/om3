package commoncmd

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/daemon/daemonenv"
)

// AuditSubsystems are the names a daemon audit selects its log feed
// with, as the subsystems register them.
//
// The ones ending with a colon take a suffix: the object path for
// icfg, imon and omon, the heartbeat id for hb. A name is matched as a
// glob pattern, so "imon:*" audits every object monitor and "hb.*"
// every heartbeat component but the streams themselves.
var AuditSubsystems = []string{
	daemonenv.ListenerNameFamily,
	daemonenv.ListenerNameInet,
	daemonenv.ListenerNameUX,
	"ccfg",
	"collector",
	"cstat",
	"daemonauth",
	"daemondata",
	"discover",
	"dns",
	"hb",
	"hb.ctrl",
	"hb.main",
	"hb.peer_dropper",
	"hb:",
	"hook",
	"icfg",
	"icfg:",
	"imon",
	"imon:",
	"istat",
	"mntmon",
	"netmon",
	"nmon",
	"omon",
	"omon:",
	"pubsub",
	"runner",
	"scheduler",
}

// auditSubsystemsHelp renders the subsystem names as indented lines no
// wider than a narrow terminal, for a command Long description.
func auditSubsystemsHelp() string {
	var (
		sb   strings.Builder
		line string
	)
	for _, name := range AuditSubsystems {
		switch {
		case line == "":
			line = "  " + name
		case len(line)+1+len(name) > 76:
			sb.WriteString(line + "\n")
			line = "  " + name
		default:
			line += " " + name
		}
	}
	if line != "" {
		sb.WriteString(line + "\n")
	}
	return sb.String()
}

// validAuditSubsystems completes the names an audit may select.
func validAuditSubsystems(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return filterPrefix(AuditSubsystems, toComplete), cobra.ShellCompDirectiveNoFileComp
}

// validListenerNames completes the listeners an action may address.
func validListenerNames(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return filterPrefix(daemonenv.ListenerNames, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func filterPrefix(candidates []string, toComplete string) []string {
	if toComplete == "" {
		return candidates
	}
	l := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if strings.HasPrefix(candidate, toComplete) {
			l = append(l, candidate)
		}
	}
	return l
}
