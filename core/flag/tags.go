package flag

var Tags = map[string]Opt{
	"color": Opt{
		Long:    "color",
		Default: "auto",
		Desc:    "output colorization yes|no|auto",
	},
	"config": Opt{
		Long: "config",
		Desc: "the configuration to use as template when creating or installing a service. the value can be `-` or `/dev/stdin` to read the json-formatted configuration from stdin, or a file path, or uri pointing to a ini-formatted configuration, or a service selector expression (ATTENTION with cloning existing live services that include more than containers, volumes and backend ip addresses ... this could cause disruption on the cloned service)",
	},
	"confirm": Opt{
		Long: "confirm",
		Desc: "confirm a run action configured to ask for confirmation. this can be used when scripting the run or triggering it from the api.",
	},
	"cron": Opt{
		Long: "cron",
		Desc: "run the action as if executed by the daemon. for example, the run action requirements error message are disabled.",
	},
	"disable-rollback": Opt{
		Long: "disable-rollback",
		Desc: "on action error, do not return activated resources to their previous state",
	},
	"discard": Opt{
		Long: "discard",
		Desc: "discard the stashed, invalid, configuration file leftover of a previous execution",
	},
	"downto": Opt{
		Long:       "downto",
		Desc:       "stop the service down to the specified rid or driver group",
		Deprecated: "use --to",
	},
	"driver": Opt{
		Long: "driver",
		Desc: "a driver identifier, <group>.<name> (ex: ip.host)",
	},
	"dry-run": Opt{
		Long: "dry-run",
		Desc: "show the action execution plan",
	},
	"env": Opt{
		Long: "env",
		Desc: "export the uppercased variable in the os environment. with the create action only, set a env section parameter in the service configuration file. multiple `--env <key>=<val>` can be specified",
	},
	"eval": Opt{
		Long: "eval",
		Desc: "dereference and evaluate arythmetic expressions in value",
	},
	"format": Opt{
		Long:    "format",
		Default: "auto",
		Desc:    "output format json|flat|auto",
	},
	"force": Opt{
		Long: "force",
		Desc: "allow dangerous operations",
	},
	"impersonate": Opt{
		Long: "impersonate",
		Desc: "the name of a peer node to impersonate when evaluating keywords",
	},
	"interactive": Opt{
		Long: "interactive",
		Desc: "prompt the user for env keys override values. fail if no default is defined",
	},
	"key": Opt{
		Long: "key",
		Desc: "a keystore key name",
	},
	"kw": Opt{
		Long: "kw",
		Desc: "a configuration keyword, [<section>].<option>",
	},
	"kwops": Opt{
		Long: "kw",
		Desc: "keyword operations, <k><op><v> with op in = |= += -= ^=",
	},
	"kws": Opt{
		Long: "kw",
		Desc: "keyword list",
	},
	"leader": Opt{
		Long: "leader",
		Desc: "provision all resources, including shared resources that are not provisioned by default",
	},
	"local": Opt{
		Long: "local",
		Desc: "inline action on local instance",
	},
	"createnamespace": Opt{
		Long: "namespace",
		Desc: "where to create the new objects",
	},
	"match": Opt{
		Long:    "match",
		Desc:    "a fnmatch key name filter",
		Default: "**",
	},
	"networkstatusname": Opt{
		Long: "name",
		Desc: "filter on a network name",
	},
	"networkstatusverbose": Opt{
		Long: "verbose",
		Desc: "include network addresses",
	},
	"node": Opt{
		Long: "node",
		Desc: "execute on a list of nodes",
	},
	"nolock": Opt{
		Long: "nolock",
		Desc: "don't acquire the action lock (danger)",
	},
	"object": Opt{
		Long:  "service",
		Short: "s",
		Desc:  "execute on a list of objects",
	},
	"objselector": Opt{
		Long:    "selector",
		Short:   "s",
		Default: "",
		Desc:    "an object selector expression, '**/s[12]+!*/vol/*'",
	},
	"poolstatusname": Opt{
		Long: "name",
		Desc: "filter on a pool name",
	},
	"poolstatusverbose": Opt{
		Long: "verbose",
		Desc: "include pool volumes",
	},
	"provision": Opt{
		Long: "provision",
		Desc: "provision the object after create",
	},
	"recover": Opt{
		Long: "recover",
		Desc: "recover the stashed, invalid, configuration file leftover of a previous execution",
	},
	"refresh": Opt{
		Long:  "refresh",
		Short: "r",
		Desc:  "refresh the status data",
	},
	"restore": Opt{
		Long: "restore",
		Desc: "keep the same object id as the origin template or config file. the default is to generate a new id",
	},
	"devroles": Opt{
		Long:    "roles",
		Desc:    "display only devices matching these roles all=exposed,sub,base",
		Default: "all",
	},
	"rid": Opt{
		Long: "rid",
		Desc: "resource selector expression (ip#1,app,disk.type=zvol)",
	},
	"server": Opt{
		Long: "server",
		Desc: "uri of the opensvc api server. scheme raw|https",
	},
	"time": Opt{
		Long:    "time",
		Default: "5m",
		Desc:    "stop waiting for the object to reach the target state after a duration",
	},
	"collector_user": Opt{
		Long: "user",
		Desc: "authenticate with the collector using the specified user credentials instead of the node credentials. required with 'om node register' when the collector is configured to refuse anonymous register",
	},
	"collector_password": Opt{
		Long: "password",
		Desc: "authenticate with the collector using the specified user credentials instead of the node credentials. prompted if necessary but not specified",
	},
	"collector_app": Opt{
		Long: "app",
		Desc: "optional. register the node in the specified app. if not specified, the node is registered in the first registering user's app found",
	},
	"moduleset": Opt{
		Long:    "moduleset",
		Desc:    "the modulesets to limit the action to. The special value `all` can be used in conjonction with detach.",
		Aliases: []string{"modulesets"},
	},
	"ruleset": Opt{
		Long:    "ruleset",
		Desc:    "the rulesets to limit the action to. The special value `all` can be used in conjonction with detach.",
		Aliases: []string{"rulesets"},
	},
	"subset": Opt{
		Long:    "subset",
		Desc:    "subset selector expression (g1,g2)",
		Aliases: []string{"subsets"},
	},
	"template": Opt{
		Long: "template",
		Desc: "the configuration file template name or id, served by the collector",
	},
	"to": Opt{
		Long: "to",
		Desc: "start or stop the service until the specified rid or driver group included",
	},
	"tags": Opt{
		Long: "tags",
		Desc: "tag selector expression (t1,t2)",
	},
	"upto": Opt{
		Long:       "upto",
		Desc:       "start the service up to the specified rid or driver group",
		Deprecated: "use --to",
	},
	"from": Opt{
		Long: "from",
		Desc: "the key value source (uri, file, /dev/stdin)",
	},
	"value": Opt{
		Long: "value",
		Desc: "the key value",
	},
	"wait": Opt{
		Long: "wait",
		Desc: "wait for the object to reach the target state",
	},
	"waitlock": Opt{
		Long:    "waitlock",
		Default: "30s",
		Desc:    "lock acquire timeout",
	},
	"watch": Opt{
		Long:  "watch",
		Short: "w",
		Desc:  "watch the monitor changes",
	},
}
