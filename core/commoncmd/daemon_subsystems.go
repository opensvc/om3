package commoncmd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/clusterhb"
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

// validListenerNames completes the listeners a start, stop or restart may
// address, which the unix socket listener is not one of.
func validListenerNames(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return filterPrefix(daemonenv.LifecycleListenerNames, toComplete), cobra.ShellCompDirectiveNoFileComp
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

// HeartbeatNameHelp is the NAME paragraph of the actions addressing a
// heartbeat as a whole, which all take the same argument.
const HeartbeatNameHelp = `NAME is a heartbeat: the index of a hb#<index> section of the cluster
configuration, "1" for "hb#1". The "hb#" prefix the ID column of
"om daemon hb status" shows is accepted too. A heartbeat the node does
not configure is refused.`

// HeartbeatStreamNameHelp is the NAME paragraph of the actions addressing
// one direction of a heartbeat, which is a component of its own.
const HeartbeatStreamNameHelp = `NAME is a heartbeat stream: the index of a hb#<index> section of the
cluster configuration, suffixed with .rx for the receiver or .tx for the
sender, "1.rx" for the receiver of "hb#1". The "hb#" prefix the ID column
of "om daemon hb status" shows is accepted too. A stream the node does
not configure is refused.`

// ListenerNameHelp is the NAME paragraph of the listener lifecycle actions,
// naming the listeners they may be addressed to. There are only two
// listeners, and they are the same on every node, so they are named here
// rather than looked up.
//
// The unix socket listener is named too, saying why it is not a value: a
// reader who knows it exists, from an audit or from a status, would take its
// absence for an oversight.
var ListenerNameHelp = `NAME is:

  ` + daemonenv.ListenerNameInet + `  the listener serving the tcp port

` + daemonenv.ListenerNameUX + `, the listener serving the unix socket, has no start, stop or restart
of its own: it lives as long as the daemon does, and the request asking
for it travels through it. Restart the daemon to restart it.`

// HeartbeatNames returns the heartbeats the local cluster configuration
// defines, named as the actions take them: "1" for the "hb#1" section.
func HeartbeatNames() []string {
	n, err := clusterhb.New()
	if err != nil {
		return nil
	}
	sections := n.HbNames()
	l := make([]string, 0, len(sections))
	for _, section := range sections {
		l = append(l, strings.TrimPrefix(section, "hb#"))
	}
	return l
}

// HeartbeatStreamNames returns the two streams of each heartbeat the local
// cluster configuration defines.
func HeartbeatStreamNames() []string {
	names := HeartbeatNames()
	l := make([]string, 0, len(names)*2)
	for _, name := range names {
		l = append(l, name+".rx", name+".tx")
	}
	return l
}

// validHeartbeatNames completes the heartbeats an action may address.
func validHeartbeatNames(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return filterPrefix(HeartbeatNames(), toComplete), cobra.ShellCompDirectiveNoFileComp
}

// validHeartbeatStreamNames completes the heartbeat streams an action may
// address.
func validHeartbeatStreamNames(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return filterPrefix(HeartbeatStreamNames(), toComplete), cobra.ShellCompDirectiveNoFileComp
}

// omWord matches the name of the om command where a help text names it,
// which the word boundaries keep apart from a word ending with it.
var omWord = regexp.MustCompile(`\bom\b`)

// ForProgram renders a help text written for the om command under the name
// the binary was invoked as, so an ox help does not tell its reader to type
// om. The commands of the two binaries are built here, from one source, and
// the root command names itself the same way.
func ForProgram(s string) string {
	name := filepath.Base(os.Args[0])
	if name == "om" {
		return s
	}
	return omWord.ReplaceAllString(s, name)
}
