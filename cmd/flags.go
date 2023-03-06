package cmd

import (
	"time"

	"github.com/spf13/pflag"

	"github.com/opensvc/om3/core/commands"
)

func addFlagsGlobal(flagSet *pflag.FlagSet, p *commands.OptsGlobal) {
	flagSet.StringVar(&p.Color, "color", "auto", "Output colorization yes|no|auto")
	flagSet.StringVar(&p.Format, "format", "auto", "Output format json|flat|auto")
	flagSet.StringVar(&p.Server, "server", "", "URI of the opensvc api server. scheme raw|https")
	flagSet.BoolVar(&p.Local, "local", false, "Inline action on local instance")
	flagSet.StringVar(&p.NodeSelector, "node", "", "Execute on a list of nodes")
	flagSet.StringVarP(&p.ObjectSelector, "service", "s", "", "Execute on a list of objects")
}

func addFlagsAsync(flagSet *pflag.FlagSet, p *commands.OptsAsync) {
	addFlagTime(flagSet, &p.Time)
	addFlagWait(flagSet, &p.Wait)
	addFlagWatch(flagSet, &p.Watch)
}

func addFlagsResourceSelector(flagSet *pflag.FlagSet, p *commands.OptsResourceSelector) {
	addFlagRID(flagSet, &p.RID)
	addFlagSubset(flagSet, &p.Subset)
	addFlagTag(flagSet, &p.Tag)
}

func addFlagsLock(flagSet *pflag.FlagSet, p *commands.OptsLock) {
	addFlagNoLock(flagSet, &p.Disable)
	addFlagWaitLock(flagSet, &p.Timeout)
}

func addFlagsTo(flagSet *pflag.FlagSet, p *commands.OptTo) {
	addFlagTo(flagSet, &p.To)
	addFlagUpTo(flagSet, &p.UpTo)
	addFlagDownTo(flagSet, &p.DownTo)
}

func addFlagComplianceAttach(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "attach", false, "Attach the modulesets selected for the compliance run.")
}

func addFlagComplianceForce(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "force", false, "Don't check before fix.")
}

func addFlagCpuProfile(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "cpuprofile", "", "Dump a cpu pprof in this file on exit.")
}

func addFlagCreateConfig(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "config", "", "The configuration to use as template when creating or installing a service. The value can be `-` or `/dev/stdin` to read the json-formatted configuration from stdin, or a file path, or uri pointing to a ini-formatted configuration, or a service selector expression (ATTENTION with cloning existing live services that include more than containers, volumes and backend ip addresses ... this could cause disruption on the cloned service), or a template numeric id, or template://<name>.")
}

func addFlagConfirm(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "confirm", false, "Confirm a run action configured to ask for confirmation. This can be used when scripting the run or triggering it from the api.")
}

func addFlagCreateForce(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "force", false, "Allow overwriting existing configuration files. Beware: changing the configuration of a live monitored service may cause a monitor action.")
}

func addFlagCreateNamespace(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "namespace", "", "Where to create the new objects.")
}

func addFlagCreateRestore(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "restore", false, "Keep the object id defined in the source config.")
}

func addFlagCron(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "cron", false, "Run the action as if executed by the daemon. For example, the run action requirements error message are disabled.")
}

func addFlagDevRoles(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "roles", "all", "Display only devices matching these roles all=exposed,sub,base.")
}

func addFlagDisableRollback(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "disable-rollback", false, "On action error, do not return activated resources to their previous state.")
}

func addFlagDiscard(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "discard", false, "Discard the stashed, invalid, configuration file leftover of a previous execution.")
}

func addFlagDownTo(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "downto", "", "Stop down to the specified rid or driver group.")
	flagSet.Lookup("downto").Deprecated = "Use --to."
}

func addFlagDriver(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "driver", "", "A driver identifier, <group>.<name> (ex: ip.host).")
}

func addFlagDryRun(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "dry-run", false, "Show the action execution plan.")
}

func addFlagDuration(flagSet *pflag.FlagSet, p *time.Duration) {
	flagSet.DurationVar(p, "duration", 0*time.Second, "duration.")
}

func addFlagEnv(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "env", "", "Export the uppercased variable in the os environment. With the create action only, set a env section parameter in the service configuration file. Multiple `--env <key>=<val>` can be specified.")
}

func addFlagDebug(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "debug", false, "Activate debug mode.")
}

func addFlagEval(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "eval", false, "Dereference and evaluate arythmetic expressions in value.")
}

func addFlagEventFilters(flagSet *pflag.FlagSet, p *[]string) {
	flagSet.StringArrayVar(p, "filter", []string{}, "request only events matching kind (InstanceStatusUpdated) or labels (path=svc1) or both (InstanceStatusUpdated,path=svc1,node=n1).")
}

func addFlagForeground(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVarP(p, "foreground", "f", false, "Restart the daemon in foreground mode.")
}

func addFlagForce(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "force", false, "Allow dangerous operations.")
}

func addFlagImpersonate(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "impersonate", "", "The name of a peer node to impersonate when evaluating keywords.")
}

func addFlagInteractive(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "interactive", false, "Prompt the user for env keys override values. Fail if no default is defined.")
}

func addFlagKey(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "key", "", "A keystore key name.")
}

func addFlagKeyword(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "kw", "", "A configuration keyword, [<section>].<option>.")
}

func addFlagKeywordOps(flagSet *pflag.FlagSet, p *[]string) {
	flagSet.StringSliceVar(p, "kw", []string{}, "A configuration keyword operation, [<section>].<option><op><value>, with op in = |= += -= ^=.")
}

func addFlagKeywords(flagSet *pflag.FlagSet, p *[]string) {
	flagSet.StringSliceVar(p, "kw", []string{}, "A configuration keywords, [<section>].<option>.")
}

func addFlagLeader(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "leader", false, "Provision all resources, including shared resources that must be provisioned only once.")
}

func addFlagLocal(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "local", false, "Inline action on local instance.")
}

func addFlagLogsFollow(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVarP(p, "logs-follow", "f", false, "Follow the log feed.")
}

func addFlagLogsSID(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "sid", "", "Filter on the session id of an action.")
}

func addFlagMatch(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "match", "**", "A fnmatch key name filter.")
}

func addFlagNetworkStatusName(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "name", "", "Filter on a network name.")
}

func addFlagNetworkStatusVerbose(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "verbose", false, "Include network addresses.")
}

func addFlagNode(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "node", "", "Execute on a list of nodes.")
}

func addFlagNoLock(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "no-lock", false, "Don't acquire the action lock (danger).")
}

func addFlagObject(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVarP(p, "service", "s", "", "Execute on a list of objects.")
}

func addFlagObjectSelector(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVarP(p, "service", "s", "", "An object selector expression. `**/s[12]+!*/vol/*`.")
}

func addFlagPoolStatusName(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "name", "", "Filter on a pool name.")
}

func addFlagPoolStatusVerbose(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "verbose", false, "Include pool volumes.")
}

func addFlagProvision(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "provision", false, "Provision the object after create.")
}

func addFlagRefresh(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVarP(p, "refresh", "r", false, "Refresh the status data.")
}

func addFlagRecover(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "recover", false, "Recover the stashed, invalid, configuration file leftover of a previous execution.")
}

func addFlagRelay(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "relay", "", "The name of the relay to query. If not specified, all known relays are queried.")
}

func addFlagRID(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "rid", "", "Resource selector expression (ip#1,app,disk.type=zvol).")
}

func addFlagTime(flagSet *pflag.FlagSet, p *time.Duration) {
	flagSet.DurationVar(p, "time", 5*time.Minute, "Stop waiting for the object to reach the target state after a duration.")
}

func addFlagCollectorUser(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "user", "", "Authenticate with the collector using the specified user credentials instead of the node credentials. Required with 'om node register' when the collector is configured to refuse anonymous register.")
}

func addFlagCollectorPassword(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "password", "", "Authenticate with the collector using the specified user credentials instead of the node credentials. Prompted if necessary but not specified.")
}

func addFlagCollectorApp(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "app", "", "Register the node in the specified app. If not specified, the node is registered in any app owned by the registering user.")
}

func addFlagModule(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "module", "", "The modules to limit the action to. the modules must be in already attached modulesets.")
}

func addFlagModuleset(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "moduleset", "", "The modulesets to limit the action to. The special value `all` can be used in conjonction with detach.")
}

func addFlagRoles(flagSet *pflag.FlagSet, p *[]string) {
	flagSet.StringArrayVar(p, "role", []string{}, "Api role. Example 'root'")
}

func addFlagRuleset(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "ruleset", "", "the rulesets to limit the action to. the special value `all` can be used in conjonction with detach.")
}

func addFlagSections(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "sections", "", "Sections to include in the output. threads,nodes,objects")
}

func addFlagSubset(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "subset", "", "A subset selector expression (g1,g2).")
}

func addFlagSwitchTo(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "to", "", "The remote node to start or migrate the service to.")
}

func addFlagTo(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "to", "", "start or stop the service until the specified rid or driver group included.")
}

func addFlagTag(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "tag", "", "A tag selector expression (t1,t2).")
}

func addFlagTarget(flagSet *pflag.FlagSet, p *[]string) {
	flagSet.StringSliceVar(p, "target", []string{}, "The peers to sync to. The value can be either nodes or drpnodes. If not set, all nodes and drpnodes are synchronized.")
}

func addFlagUpTo(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "upto", "", "Start up to the specified rid or driver group.")
	flagSet.Lookup("upto").Deprecated = "Use --to."
}

func addFlagFrom(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "from", "", "The key value source (uri, file, /dev/stdin).")
}

func addFlagValue(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "value", "", "The key value.")
}

func addFlagWait(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "wait", false, "Wait for the object to reach the target state.")
}

func addFlagWaitLock(flagSet *pflag.FlagSet, p *time.Duration) {
	flagSet.DurationVar(p, "waitlock", 30*time.Second, "Lock acquire timeout.")
}

func addFlagWatch(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVarP(p, "watch", "w", false, "Watch the monitor changes.")
}
