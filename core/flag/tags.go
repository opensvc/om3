package flag

var Tags = map[string]Opt{
	"color": Opt{
		Long:    "color",
		Default: "auto",
		Desc:    "output colorization yes|no|auto",
	},
	"downto": Opt{
		Long: "downto",
		Desc: "stop the service down to the specified rid or driver group",
	},
	"upto": Opt{
		Long: "upto",
		Desc: "start the service up to the specified rid or driver group",
	},
	"format": Opt{
		Long:    "format",
		Default: "auto",
		Desc:    "output format json|flat|auto"},
	"objselector": Opt{
		Long:    "selector",
		Short:   "s",
		Default: "",
		Desc:    "an object selector expression, '**/s[12]+!*/vol/*'"},
	"nolock": Opt{
		Long: "nolock",
		Desc: "don't acquire the action lock (danger)",
	},
	"time": Opt{
		Long:    "time",
		Default: "5m",
		Desc:    "stop waiting for the object to reach the target state after a duration",
	},
	"waitlock": Opt{
		Long:    "waitlock",
		Default: "30s",
		Desc:    "lock acquire timeout",
	},
	"dry-run": Opt{
		Long: "dry-run",
		Desc: "show the action execution plan",
	},
	"local": Opt{
		Long: "local",
		Desc: "inline action on local instance",
	},
	"force": Opt{
		Long: "force",
		Desc: "allow dangerous operations",
	},
	"eval": Opt{
		Long: "eval",
		Desc: "dereference and evaluate arythmetic expressions in value",
	},
	"impersonate": Opt{
		Long: "impersonate",
		Desc: "the name of a peer node to impersonate when evaluating keywords",
	},
	"kws": Opt{
		Long: "kw",
		Desc: "keyword list",
	},
	"kwops": Opt{
		Long: "kw",
		Desc: "keyword operations, <k><op><v> with op in = |= += -= ^=",
	},
	"kw": Opt{
		Long: "kw",
		Desc: "a configuration keyword, [<section>].<option>",
	},
	"object": Opt{
		Long:  "service",
		Short: "s",
		Desc:  "execute on a list of objects",
	},
	"node": Opt{
		Long: "node",
		Desc: "execute on a list of nodes",
	},
	"wait": Opt{
		Long: "wait",
		Desc: "wait for the object to reach the target state",
	},
	"watch": Opt{
		Long:  "watch",
		Short: "w",
		Desc:  "watch the monitor changes",
	},
	"refresh": Opt{
		Long:  "refresh",
		Short: "r",
		Desc:  "refresh the status data",
	},
	"rid": Opt{
		Long: "rid",
		Desc: "resource selector expression (ip#1,app,disk.type=zvol)",
	},
	"subsets": Opt{
		Long: "subsets",
		Desc: "subset selector expression (g1,g2)",
	},
	"tags": Opt{
		Long: "tags",
		Desc: "tag selector expression (t1,t2)",
	},
	"recover": Opt{
		Long: "recover",
		Desc: "recover the stashed, invalid, configuration file leftover of a previous execution",
	},
	"discard": Opt{
		Long: "discard",
		Desc: "discard the stashed, invalid, configuration file leftover of a previous execution",
	},
	"server": Opt{
		Long: "server",
		Desc: "uri of the opensvc api server. scheme raw|https",
	},
}
