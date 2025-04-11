package commoncmd

import (
	"fmt"
	"strings"
	"time"

	_ "embed"

	"github.com/opensvc/om3/daemon/rbac"
	"github.com/spf13/pflag"
)

var (
	//go:embed text/node-events/flag/template
	usageFlagEventTemplate string

	//go:embed text/node-events/flag/filter
	usageFlagEventFilter string
)

func FlagsAsync(flagSet *pflag.FlagSet, p *OptsAsync) {
	FlagTime(flagSet, &p.Time)
	FlagWait(flagSet, &p.Wait)
	FlagWatch(flagSet, &p.Watch)
}

func FlagsLogs(flagSet *pflag.FlagSet, p *OptsLogs) {
	flagSet.BoolVarP(&p.Follow, "follow", "f", false, "follow the log feed")
	flagSet.IntVarP(&p.Lines, "lines", "n", 50, "report the last n log entries")
	flagSet.StringArrayVar(&p.Filter, "filter", []string{}, "report only log entries matching labels (path=svc1)")
}

func FlagsLock(flagSet *pflag.FlagSet, p *OptsLock) {
	FlagNoLock(flagSet, &p.Disable)
	FlagWaitLock(flagSet, &p.Timeout)
}

func FlagsResourceSelector(flagSet *pflag.FlagSet, p *OptsResourceSelector) {
	FlagRID(flagSet, &p.RID)
	FlagSubset(flagSet, &p.Subset)
	FlagTag(flagSet, &p.Tag)
}

func FlagsTo(flagSet *pflag.FlagSet, p *OptTo) {
	FlagTo(flagSet, &p.To)
	FlagUpTo(flagSet, &p.UpTo)
	FlagDownTo(flagSet, &p.DownTo)
}

func FlagComplianceAttach(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "attach", false, "attach the modulesets selected for the compliance run")
}

func FlagComplianceForce(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "force", false, "don't check before fix")
}

func FlagCPUProfile(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "cpuprofile", "", "dump a cpu pprof in this file on exit")
}

func FlagCreateConfig(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "config", "", "the initial configuration source: -, /dev/stdin, file path, url, object path or template://<name>")
}

func FlagConfirm(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "confirm", false, "confirm a run action configured to ask for confirmation")
}

func FlagCreateForce(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "force", false, "allow overwriting existing configuration files (dangerous)")
}

func FlagCreateNamespace(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "namespace", "", "where to create the new objects")
}

func FlagCreateRestore(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "restore", false, "keep the object id defined in the source config")
}

func FlagCron(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "cron", false, "run the action as if executed by the daemon")
}

func FlagDaemonHeartbeatFilter(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "name", "", "filter on heartbeat name or stream name (ex: hb#1, hb#1.rx, 1, 1.rx)")
}

func FlagDaemonHeartbeatName(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "name", "", "stream name (ex: 1.rx)")
}

func FlagDaemonListenerName(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "name", "", "listener name http-inet|http-ux")
}

func FlagDaemonLogLevel(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "level", "", "trace, debug, info, warn, error, fatal, panic")
}

func FlagDepth(flagSet *pflag.FlagSet, p *int) {
	flagSet.IntVar(p, "depth", 0, "format markdown titles so they can be rooted inside a chapter nested at the specified depth")
}

func FlagDevRoles(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "roles", "all", "display only devices matching these roles all=exposed,sub,base")
}

func FlagDisableRollback(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "disable-rollback", false, "on action error, do not return activated resources to their previous state")
}

func FlagDiscard(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "discard", false, "discard the stashed, invalid, configuration file leftover of a previous execution")
}

func FlagDownTo(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "downto", "", "stop down to the specified rid or driver group")
	flagSet.Lookup("downto").Deprecated = "Use --to."
}

func FlagDriver(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "driver", "", "a driver identifier, <group>.<name> (ex: ip.host)")
}

func FlagDryRun(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "dry-run", false, "show the action execution plan")
}

func FlagDuration(flagSet *pflag.FlagSet, p *time.Duration) {
	flagSet.DurationVar(p, "duration", 0*time.Second, "duration")
}

func FlagEnv(flagSet *pflag.FlagSet, p *[]string) {
	flagSet.StringSliceVar(p, "env", []string{}, "export the variable in the action environment")
}

func FlagCreateEnv(flagSet *pflag.FlagSet, p *[]string) {
	flagSet.StringSliceVar(p, "env", []string{}, "set a env section parameter in the service configuration file")
}

func FlagEval(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "eval", false, "dereference and evaluate arythmetic expressions in value")
}

func FlagEventFilters(flagSet *pflag.FlagSet, p *[]string) {
	flagSet.StringArrayVar(p, "filter", []string{}, usageFlagEventFilter)
}

func FlagForce(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "force", false, "allow dangerous operations")
}

func FlagImpersonate(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "impersonate", "", "the name of a peer node to impersonate when evaluating keywords")
}

func FlagKey(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "key", "", "a data key name")
}

func FlagKeyTo(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "to", "", "the new data key name")
}

func FlagKeyword(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "kw", "", "a configuration keyword: [<section>.]<option>")
}

func FlagKeywordOps(flagSet *pflag.FlagSet, p *[]string) {
	flagSet.StringSliceVar(p, "kw", []string{}, "a configuration keyword operation: [<section>.]<option><op><value>, with op in = |= += -= ^=")
}

func FlagKeywords(flagSet *pflag.FlagSet, p *[]string) {
	flagSet.StringSliceVar(p, "kw", []string{}, "a configuration keyword: [<section>.]<option>")
}

func FlagLeader(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "leader", false, "provision all resources, including shared resources that must be provisioned only once")
}

func FlagLocal(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "local", false, "inline action on local instance")
}

func FlagMatch(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "match", "**", "a fnmatch key name filter")
}

func FlagNetworkStatusName(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "name", "", "filter on a network name")
}

func FlagNetworkStatusExtended(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVarP(p, "extended", "x", false, "include network addresses")
}

func FlagNodeSelectorFilter(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "node", "", "filter on a list of nodes (ex: *, az=fr1)")
}

func FlagPeerSelectorFilter(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "peer", "", "filter on a list of remote nodes (ex: *, az=fr1)")
}

func FlagNodeSelector(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "node", "", "execute on a list of nodes")
}

func FlagNoLock(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "no-lock", false, "don't acquire the action lock (dangerous)")
}

func FlagPoolName(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "name", "", "filter on a pool name")
}

func FlagPoolStatusExtended(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVarP(p, "extended", "x", false, "include pool volumes")
}

func FlagProvision(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "provision", false, "provision the object after create")
}

func FlagRefresh(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVarP(p, "refresh", "r", false, "refresh the status data")
}

func FlagRecover(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "recover", false, "recover the stashed, invalid, configuration file leftover of a previous execution")
}

func FlagRelay(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "relay", "", "the name of a relay to query")
}

func FlagRID(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "rid", "", "a resource selector expression (ex: ip#1,app,disk.type=zvol)")
}

func FlagEventTemplate(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "template", "", usageFlagEventTemplate)
}

func FlagTime(flagSet *pflag.FlagSet, p *time.Duration) {
	flagSet.DurationVar(p, "time", 5*time.Minute, "stop waiting for the object to reach the target state after a duration")
}

func FlagCollectorUser(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "user", "", "authenticate with the collector using this user")
}

func FlagCollectorPassword(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "password", "", "authenticate with the collector using this password")
}

func FlagCollectorApp(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "app", "", "register the node in the this app (or the collector picks a random app owned by the user)")
}

func FlagModule(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "module", "", "the attached modules to limit the action to")
}

func FlagModuleset(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "moduleset", "", "the modulesets to limit the action to (ex: modset1, all)")
}

func FlagOutputSections(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "sections", "", "sections to include in the output (ex: threads,nodes,objects)")
}

func FlagRoles(flagSet *pflag.FlagSet, p *[]string) {
	flagSet.StringSliceVar(p, "role", nil, fmt.Sprintf("roles to include as a token claim (ex: %s)", strings.Join(rbac.Roles(), ",")))
}

func FlagRuleset(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "ruleset", "", "the rulesets to limit the action to (ex: rset1, all)")
}

func FlagSections(flagSet *pflag.FlagSet, p *[]string) {
	flagSet.StringSliceVar(p, "section", []string{}, "a configuration section")
}

func FlagSubset(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "subset", "", "a subset selector expression (ex: g1,g2)")
}

func FlagSwitchTo(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "to", "", "a remote node to switch the service to")
}

func FlagTo(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "to", "", "start or stop the service until this rid or driver group is done")
}

func FlagTag(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "tag", "", "a tag selector expression (ex: t1,t2)")
}

func FlagTarget(flagSet *pflag.FlagSet, p *[]string) {
	flagSet.StringSliceVar(p, "target", []string{}, "the peers to sync to (ex: nodes or drpnodes)")
}

func FlagUpdateDelete(flagSet *pflag.FlagSet, p *[]string) {
	flagSet.StringSliceVar(p, "delete", []string{}, "a configuration section to delete")
}

func FlagUpdateSet(flagSet *pflag.FlagSet, p *[]string) {
	flagSet.StringSliceVar(p, "set", []string{}, "a keyword operation to apply to the configuration")
}

func FlagUpdateUnset(flagSet *pflag.FlagSet, p *[]string) {
	flagSet.StringSliceVar(p, "unset", []string{}, "a keyword to unset from the configuration")
}

func FlagUpTo(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "upto", "", "start up to the specified rid or driver group")
	flagSet.Lookup("upto").Deprecated = "Use --to."
}

func FlagFrom(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "from", "", "the key value source (ex: uri, file, /dev/stdin)")
}

func FlagKeyName(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "name", "", "the key name")
}

func FlagKeyValue(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "value", "", "the key value")
}

func FlagWait(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "wait", false, "wait for the object to reach the target state")
}

func FlagWaitLock(flagSet *pflag.FlagSet, p *time.Duration) {
	flagSet.DurationVar(p, "waitlock", 30*time.Second, "lock acquire timeout")
}

func FlagWatch(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVarP(p, "watch", "w", false, "watch the monitor changes")
}
