package commoncmd

import (
	_ "embed"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/pflag"

	"github.com/opensvc/om3/daemon/rbac"
)

func DeprecatedFlagDownTo(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "downto", "", "DEPRECATED: stop down to the specified rid or driver group")
	flags.MarkDeprecated("downto", "Please use --to instead.")
	flags.MarkHidden("downto")
}

func DeprecatedFlagSubsets(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "subsets", "", "DEPRECATED: a subset selector expression (ex: g1,g2)")
	flags.MarkDeprecated("subsets", "Please use --subset instead.")
	flags.MarkHidden("subsets")
}

func DeprecatedFlagTags(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "tags", "", "DEPRECATED: a tag selector expression (ex: t1,t2)")
	flags.MarkDeprecated("tags", "Please use --tag instead.")
	flags.MarkHidden("tags")
}

func DeprecatedFlagUpTo(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "upto", "", "DEPRECATED: start up to the specified rid or driver group")
	flags.MarkDeprecated("upto", "Please use --to instead.")
	flags.MarkHidden("upto")
}

func FlagsAsync(flags *pflag.FlagSet, p *OptsAsync) {
	FlagTime(flags, &p.Time)
	FlagWait(flags, &p.Wait)
	FlagWatch(flags, &p.Watch)
}

func FlagsLogs(flags *pflag.FlagSet, p *OptsLogs) {
	flags.BoolVarP(&p.Follow, "follow", "f", false, "follow the log feed")
	flags.IntVarP(&p.Lines, "lines", "n", 50, "report the last n log entries")
	flags.StringArrayVar(&p.Filter, "filter", []string{}, "report only log entries matching labels (path=svc1)")
}

func FlagsLock(flags *pflag.FlagSet, p *OptsLock) {
	FlagNoLock(flags, &p.Disable)
	FlagWaitLock(flags, &p.Timeout)
}

func FlagsEncap(flags *pflag.FlagSet, p *OptsEncap) {
	FlagSlave(flags, &p.Slaves)
	FlagSlaves(flags, &p.AllSlaves)
	FlagMaster(flags, &p.Master)
}

func FlagsResourceSelector(flags *pflag.FlagSet, p *OptsResourceSelector) {
	DeprecatedFlagSubsets(flags, &p.Tag)
	DeprecatedFlagTags(flags, &p.Tag)
	FlagRID(flags, &p.RID)
	FlagSubset(flags, &p.Subset)
	FlagTag(flags, &p.Tag)
}

func FlagsTo(flags *pflag.FlagSet, p *OptTo) {
	FlagTo(flags, &p.To)
	DeprecatedFlagUpTo(flags, &p.UpTo)
	DeprecatedFlagDownTo(flags, &p.DownTo)
}

func FlagComplianceAttach(flags *pflag.FlagSet, p *bool) {
	flags.BoolVar(p, "attach", false, "attach the modulesets selected for the compliance run")
}

func FlagComplianceForce(flags *pflag.FlagSet, p *bool) {
	flags.BoolVar(p, "force", false, "don't check before fix")
}

func FlagConfirm(flags *pflag.FlagSet, p *bool) {
	flags.BoolVar(p, "confirm", false, "confirm a run action configured to ask for confirmation")
}

func FlagCPUProfile(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "cpuprofile", "", "dump a cpu pprof in this file on exit")
}

func FlagCreateConfig(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "config", "", "the initial configuration source: -, /dev/stdin, file path, url, object path or template://<name>")
}

func FlagCreateForce(flags *pflag.FlagSet, p *bool) {
	flags.BoolVar(p, "force", false, "allow overwriting existing configuration files (dangerous)")
}

func FlagCreateNamespace(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "namespace", "", "where to create the new objects")
}

func FlagCreateRestore(flags *pflag.FlagSet, p *bool) {
	flags.BoolVar(p, "restore", false, "keep the object id defined in the source config")
}

func FlagCron(flags *pflag.FlagSet, p *bool) {
	flags.BoolVar(p, "cron", false, "run the action as if executed by the daemon")
}

func FlagDaemonHeartbeatFilter(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "name", "", "filter on heartbeat name or stream name (ex: hb#1, hb#1.rx, 1, 1.rx)")
}

func FlagDaemonHeartbeatName(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "name", "", "stream name (ex: 1.rx)")
}

func FlagDaemonListenerName(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "name", "", "listener name http-inet|http-ux")
}

func FlagDaemonLogLevel(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "level", "", "trace, debug, info, warn, error, fatal, panic")
}

func FlagDepth(flags *pflag.FlagSet, p *int) {
	flags.IntVar(p, "depth", 0, "format markdown titles so they can be rooted inside a chapter nested at the specified depth")
}

func FlagDevRoles(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "roles", "all", "display only devices matching these roles all=exposed,sub,base")
}

func FlagDisableRollback(flags *pflag.FlagSet, p *bool) {
	flags.BoolVar(p, "disable-rollback", false, "on action error, do not return activated resources to their previous state")
}

func FlagDiscard(flags *pflag.FlagSet, p *bool) {
	flags.BoolVar(p, "discard", false, "discard the stashed, invalid, configuration file leftover of a previous execution")
}

func FlagDriver(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "driver", "", "a driver identifier, <group>.<name> (ex: ip.host)")
}

func FlagDryRun(flags *pflag.FlagSet, p *bool) {
	flags.BoolVar(p, "dry-run", false, "show the action execution plan")
}

func FlagDuration(flags *pflag.FlagSet, p *time.Duration) {
	flags.DurationVar(p, "duration", 0*time.Second, "duration")
}

func FlagEnv(flags *pflag.FlagSet, p *[]string) {
	flags.StringSliceVar(p, "env", []string{}, "export the variable in the action environment")
}

func FlagCreateEnv(flags *pflag.FlagSet, p *[]string) {
	flags.StringSliceVar(p, "env", []string{}, "set a env section parameter in the service configuration file")
}

func FlagEval(flags *pflag.FlagSet, p *bool) {
	flags.BoolVar(p, "eval", false, "dereference and evaluate arythmetic expressions in value")
}

func FlagEventFilters(flags *pflag.FlagSet, p *[]string) {
	flags.StringArrayVar(p, "filter", []string{}, "filter events on kind, labels and data (see above)")
}

func FlagEventOutput(flags *pflag.FlagSet, p *string) {
	flags.StringVarP(p, "output", "o", "auto", "output format json|flat|diff|auto|tab=<header>:<jsonpath>,...|template=<go template>")
	flags.StringVar(p, "format", "auto", "output format json|flat|diff|auto|tab=<header>:<jsonpath>,...|template=<go template>")
	flags.MarkHidden("format")
}

func FlagEventTemplate(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "template", "", "a go template with custom functions (see above)")
}

func FlagForce(flags *pflag.FlagSet, p *bool) {
	flags.BoolVar(p, "force", false, "allow dangerous operations")
}

func FlagImpersonate(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "impersonate", "", "the name of a peer node to impersonate when evaluating keywords")
}

func FlagKey(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "key", "", "a data key name")
}

func FlagKeyTo(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "to", "", "the new data key name")
}

func FlagKeyword(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "kw", "", "a configuration keyword: [<section>.]<option>")
}

func FlagKeywordOps(flags *pflag.FlagSet, p *[]string) {
	flags.StringSliceVar(p, "kw", []string{}, "a configuration keyword operation: [<section>.]<option><op><value>, with op in = |= += -= ^=")
}

func FlagKeywords(flags *pflag.FlagSet, p *[]string) {
	flags.StringSliceVar(p, "kw", []string{}, "a configuration keyword: [<section>.]<option>")
}

func FlagLeader(flags *pflag.FlagSet, p *bool) {
	flags.BoolVar(p, "leader", false, "provision all resources, including shared resources that must be provisioned only once")
}

func FlagLive(flags *pflag.FlagSet, p *bool) {
	flags.BoolVar(p, "live", false, "use live migration if possible")
}

func FlagLocal(flags *pflag.FlagSet, p *bool) {
	flags.BoolVar(p, "local", false, "inline action on local instance")
}

func FlagMatch(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "match", "**", "a fnmatch key name filter")
}

func FlagNetworkStatusName(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "name", "", "filter on a network name")
}

func FlagNetworkStatusExtended(flags *pflag.FlagSet, p *bool) {
	flags.BoolVarP(p, "extended", "x", false, "include network addresses")
}

func FlagNodeSelectorFilter(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "node", "", "filter on a list of nodes (ex: *, az=fr1)")
}

func FlagPeerSelectorFilter(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "peer", "", "filter on a list of remote nodes (ex: *, az=fr1)")
}

func FlagNodeSelector(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "node", "", "submit the action to the selected nodes")
}

func FlagNodeSelectorOrLocalhost(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "node", "localhost", "submit the action to the selected nodes")
}

func FlagNodeSelectorOrAll(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "node", "*", "submit the action to the selected nodes")
}

func FlagNoLock(flags *pflag.FlagSet, p *bool) {
	flags.BoolVar(p, "no-lock", false, "don't acquire the action lock (dangerous)")
}

func FlagPoolName(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "name", "", "filter on a pool name")
}

func FlagPoolStatusExtended(flags *pflag.FlagSet, p *bool) {
	flags.BoolVarP(p, "extended", "x", false, "include pool volumes")
}

func FlagProvision(flags *pflag.FlagSet, p *bool) {
	flags.BoolVar(p, "provision", false, "provision the object after create")
}

func FlagRefresh(flags *pflag.FlagSet, p *bool) {
	flags.BoolVarP(p, "refresh", "r", false, "refresh the status data")
}

func FlagRecover(flags *pflag.FlagSet, p *bool) {
	flags.BoolVar(p, "recover", false, "recover the stashed, invalid, configuration file leftover of a previous execution")
}

func FlagRelay(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "relay", "", "the name of a relay to query")
}

func FlagRID(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "rid", "", "a resource selector expression (ex: ip#1,app,disk.type=zvol)")
}

func FlagStateOnly(flags *pflag.FlagSet, p *bool) {
	flags.BoolVar(p, "state-only", false, "change only internal state")
}

func FlagTime(flags *pflag.FlagSet, p *time.Duration) {
	flags.DurationVar(p, "time", 5*time.Minute, "stop waiting for the object to reach the target state after a duration")
}

func FlagCollectorUser(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "user", "", "authenticate with the collector using this user")
}

func FlagCollectorPassword(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "password", "", "authenticate with the collector using this password")
}

func FlagCollectorApp(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "app", "", "register the node in the this app (or the collector picks a random app owned by the user)")
}

func FlagMaster(flags *pflag.FlagSet, p *bool) {
	flags.BoolVar(p, "master", false, "do not execute on encap nodes")
}

func FlagModule(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "module", "", "the attached modules to limit the action to")
}

func FlagModuleset(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "moduleset", "", "the modulesets to limit the action to (ex: modset1, all)")
}

func FlagOutputSections(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "sections", "", "sections to include in the output (ex: threads,nodes,objects)")
}

func FlagRoles(flags *pflag.FlagSet, p *[]string) {
	flags.StringSliceVar(p, "role", nil, fmt.Sprintf("roles to include as a token claim (ex: %s)", strings.Join(rbac.Roles(), ",")))
}

func FlagRuleset(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "ruleset", "", "the rulesets to limit the action to (ex: rset1, all)")
}

func FlagSlave(flags *pflag.FlagSet, p *[]string) {
	flags.StringSliceVar(p, "slave", []string{}, "execute only on the selected encap nodes")
}

func FlagSlaves(flags *pflag.FlagSet, p *bool) {
	flags.BoolVar(p, "slaves", false, "execute only on encap nodes")
}

func FlagSections(flags *pflag.FlagSet, p *[]string) {
	flags.StringSliceVar(p, "section", []string{}, "a configuration section")
}

func FlagMoveTo(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "move-to", "", "live-migrate capable resources destination")
}

func FlagSubset(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "subset", "", "a subset selector expression (ex: g1,g2)")
}

func FlagSwitchTo(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "to", "", "a remote node to switch the service to")
}

func FlagTo(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "to", "", "process until the specified rid or driver group is done")
}

func FlagTag(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "tag", "", "a tag selector expression (ex: t1,t2)")
}

func FlagTarget(flags *pflag.FlagSet, p *[]string) {
	flags.StringSliceVar(p, "target", []string{}, "the peers to sync to (ex: nodes or drpnodes)")
}

func FlagUpdateDelete(flags *pflag.FlagSet, p *[]string) {
	flags.StringSliceVar(p, "delete", []string{}, "a configuration section to delete")
}

func FlagUpdateSet(flags *pflag.FlagSet, p *[]string) {
	flags.StringSliceVar(p, "set", []string{}, "a keyword operation to apply to the configuration")
}

func FlagUpdateUnset(flags *pflag.FlagSet, p *[]string) {
	flags.StringSliceVar(p, "unset", []string{}, "a keyword to unset from the configuration")
}

func FlagFrom(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "from", "", "the key value source (ex: uri, file, /dev/stdin)")
}

func FlagKeyName(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "name", "", "the key name")
}

func FlagKeyValue(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "value", "", "the key value")
}

func FlagEventLimit(flags *pflag.FlagSet, p *uint64) {
	flags.Uint64Var(p, "limit", 0, "exit when <limit> events are received, the default is 0 (unlimited) or 1 if --wait is set")
}

func FlagWait(flags *pflag.FlagSet, p *bool) {
	flags.BoolVar(p, "wait", false, "wait for the object to reach the target state")
}

func FlagWaitLock(flags *pflag.FlagSet, p *time.Duration) {
	flags.DurationVar(p, "waitlock", 30*time.Second, "lock acquire timeout")
}

func FlagWatch(flags *pflag.FlagSet, p *bool) {
	flags.BoolVarP(p, "watch", "w", false, "watch the monitor changes")
}

func FlagColor(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "color", "auto", "output colorization yes|no|auto")
}

func FlagOutput(flags *pflag.FlagSet, p *string) {
	flags.StringVarP(p, "output", "o", "auto", "output format json|flat|auto|tab=<header>:<jsonpath>,...")
	flags.StringVar(p, "format", "auto", "output format json|flat|auto|tab=<header>:<jsonpath>,...")
	flags.MarkHidden("format")
}

func FlagObjectSelector(flags *pflag.FlagSet, p *string) {
	flags.StringVarP(p, "service", "", "", "execute on a list of objects")
	flags.StringVarP(p, "selector", "s", "", "execute on a list of objects")
	flags.MarkHidden("service")
}

func HiddenFlagsEncap(flags *pflag.FlagSet, p *OptsEncap) {
	HiddenFlagSlave(flags, &p.Slaves)
	HiddenFlagSlaves(flags, &p.AllSlaves)
	HiddenFlagMaster(flags, &p.Master)
}

func HiddenFlagsLock(flags *pflag.FlagSet, p *OptsLock) {
	HiddenFlagNoLock(flags, &p.Disable)
	HiddenFlagWaitLock(flags, &p.Timeout)
}

func HiddenFlagsResourceSelector(flags *pflag.FlagSet, p *OptsResourceSelector) {
	HiddenFlagRID(flags, &p.RID)
	HiddenFlagSubset(flags, &p.Subset)
	HiddenFlagTag(flags, &p.Tag)
}

func HiddenFlagsTo(flags *pflag.FlagSet, p *OptTo) {
	HiddenFlagTo(flags, &p.To)
	DeprecatedFlagUpTo(flags, &p.UpTo)
	DeprecatedFlagDownTo(flags, &p.DownTo)
}

func HiddenFlagLeader(flags *pflag.FlagSet, p *bool) {
	FlagLeader(flags, p)
	flags.MarkHidden("leader")
}

func HiddenFlagDisableRollback(flags *pflag.FlagSet, p *bool) {
	FlagDisableRollback(flags, p)
	flags.MarkHidden("disable-rollback")
}

func HiddenFlagForce(flags *pflag.FlagSet, p *bool) {
	FlagForce(flags, p)
	flags.MarkHidden("force")
}

func HiddenFlagMaster(flags *pflag.FlagSet, p *bool) {
	FlagMaster(flags, p)
	flags.MarkHidden("master")
}

func HiddenFlagNodeSelector(flags *pflag.FlagSet, p *string) {
	FlagNodeSelector(flags, p)
	flags.MarkHidden("node")
}

func HiddenFlagNoLock(flags *pflag.FlagSet, p *bool) {
	FlagNoLock(flags, p)
	flags.MarkHidden("no-lock")
}

func HiddenFlagRID(flags *pflag.FlagSet, p *string) {
	FlagRID(flags, p)
	flags.MarkHidden("rid")
}

func HiddenFlagSlave(flags *pflag.FlagSet, p *[]string) {
	FlagSlave(flags, p)
	flags.MarkHidden("slave")
}

func HiddenFlagSlaves(flags *pflag.FlagSet, p *bool) {
	FlagSlaves(flags, p)
	flags.MarkHidden("slaves")
}

func HiddenFlagSubset(flags *pflag.FlagSet, p *string) {
	FlagSubset(flags, p)
	flags.MarkHidden("subset")
}

func HiddenFlagTo(flags *pflag.FlagSet, p *string) {
	FlagTo(flags, p)
	flags.MarkHidden("to")
}

func HiddenFlagTag(flags *pflag.FlagSet, p *string) {
	FlagTag(flags, p)
	flags.MarkHidden("tag")
}

func HiddenFlagWaitLock(flags *pflag.FlagSet, p *time.Duration) {
	flags.DurationVar(p, "waitlock", 30*time.Second, "lock acquire timeout")
	flags.MarkHidden("waitlock")
}

func HiddenFlagObjectSelector(flags *pflag.FlagSet, p *string) {
	FlagObjectSelector(flags, p)
	flags.MarkHidden("selector")
}
