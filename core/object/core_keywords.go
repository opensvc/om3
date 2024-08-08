package object

import (
	"embed"
	"fmt"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/placement"
	"github.com/opensvc/om3/core/priority"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/resourceid"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/util/converters"
	"github.com/opensvc/om3/util/key"
)

//go:embed text
var fs embed.FS

var keywordStore = keywords.Store{
	{
		Section:   "DEFAULT",
		Option:    "hard_affinity",
		Inherit:   keywords.InheritHead,
		Converter: converters.ListLowercase,
		Aliases:   []string{"affinity"},
		Text:      keywords.NewText(fs, "text/kw/core/hard_affinity"),
		Example:   "svc1 svc2",
	},
	{
		Section:   "DEFAULT",
		Option:    "hard_anti_affinity",
		Inherit:   keywords.InheritHead,
		Converter: converters.ListLowercase,
		Aliases:   []string{"anti_affinity"},
		Text:      keywords.NewText(fs, "text/kw/core/hard_anti_affinity"),
		Example:   "svc1 svc2",
	},
	{
		Section:   "DEFAULT",
		Option:    "soft_affinity",
		Inherit:   keywords.InheritHead,
		Converter: converters.ListLowercase,
		Text:      keywords.NewText(fs, "text/kw/core/soft_affinity"),
		Example:   "svc1 svc2",
	},
	{
		Section:   "DEFAULT",
		Option:    "soft_anti_affinity",
		Inherit:   keywords.InheritHead,
		Converter: converters.ListLowercase,
		Text:      keywords.NewText(fs, "text/kw/core/soft_anti_affinity"),
		Example:   "svc1 svc2",
	},
	{
		Section:     "DEFAULT",
		Option:      "id",
		Scopable:    false,
		DefaultText: keywords.NewText(fs, "text/kw/core/id.default"),
		Text:        keywords.NewText(fs, "text/kw/core/id"),
	},
	{
		Option: "comment",
		Text:   keywords.NewText(fs, "text/kw/core/comment"),
	},
	{
		Section:   "DEFAULT",
		Option:    "disable",
		Scopable:  true,
		Converter: converters.Bool,
		Text:      keywords.NewText(fs, "text/kw/core/disable"),
	},
	{
		Section:   "DEFAULT",
		Option:    "create_pg",
		Default:   "true",
		Scopable:  true,
		Converter: converters.Bool,
		Text:      keywords.NewText(fs, "text/kw/core/create_pg"),
	},
	{
		Option:   "pg_cpus",
		Attr:     "PG.Cpus",
		Scopable: true,
		Inherit:  keywords.InheritLeaf,
		Depends:  keyop.ParseList("create_pg=true"),
		Example:  "0-2",
		Text:     keywords.NewText(fs, "text/kw/core/pg_cpus"),
	},
	{
		Option:   "pg_mems",
		Attr:     "PG.Mems",
		Scopable: true,
		Inherit:  keywords.InheritLeaf,
		Example:  "0-2",
		Text:     keywords.NewText(fs, "text/kw/core/pg_mems"),
	},
	{
		Option:    "pg_cpu_shares",
		Attr:      "PG.CpuShares",
		Scopable:  true,
		Converter: converters.Size,
		Inherit:   keywords.InheritLeaf,
		Example:   "512",
		Text:      keywords.NewText(fs, "text/kw/core/pg_cpu_shares"),
	},
	{
		Option:   "pg_cpu_quota",
		Attr:     "PG.CpuQuota",
		Scopable: true,
		Inherit:  keywords.InheritLeaf,
		Example:  "50%@all",
		Text:     keywords.NewText(fs, "text/kw/core/pg_cpu_shares"),
	},
	{
		Option:   "pg_mem_oom_control",
		Attr:     "PG.MemOOMControl",
		Scopable: true,
		Inherit:  keywords.InheritLeaf,
		Example:  "1",
		Text:     keywords.NewText(fs, "text/kw/core/pg_mem_oom_control"),
	},
	{
		Option:    "pg_mem_limit",
		Attr:      "PG.MemLimit",
		Scopable:  true,
		Converter: converters.Size,
		Inherit:   keywords.InheritLeaf,
		Example:   "512m",
		Text:      keywords.NewText(fs, "text/kw/core/pg_mem_limit"),
	},
	{
		Option:    "pg_vmem_limit",
		Attr:      "PG.VMemLimit",
		Scopable:  true,
		Converter: converters.Size,
		Inherit:   keywords.InheritLeaf,
		Text:      keywords.NewText(fs, "text/kw/core/pg_vmem_limit"),
		Example:   "1g",
	},
	{
		Option:   "pg_mem_swappiness",
		Attr:     "PG.MemSwappiness",
		Scopable: true,
		Inherit:  keywords.InheritLeaf,
		Text:     keywords.NewText(fs, "text/kw/core/pg_mem_swappiness"),
		Example:  "40",
	},
	{
		Option:   "pg_blkio_weight",
		Attr:     "PG.BlkioWeight",
		Scopable: true,
		Inherit:  keywords.InheritLeaf,
		Text:     keywords.NewText(fs, "text/kw/core/pg_blkio_weight"),
		Example:  "50",
	},
	{
		Option:    "stat_timeout",
		Converter: converters.Duration,
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/core/stat_timeout"),
	},
	{
		Section:     "DEFAULT",
		Option:      "nodes",
		Scopable:    true,
		Kind:        naming.NewKinds(naming.KindSvc, naming.KindVol),
		Inherit:     keywords.InheritHead,
		Converter:   xconfig.NodesConverter,
		DefaultText: keywords.NewText(fs, "text/kw/core/nodes.default"),
		Text:        keywords.NewText(fs, "text/kw/core/nodes"),
		Example:     "n1 n*",
	},
	{
		Section:   "DEFAULT",
		Option:    "nodes",
		Scopable:  true,
		Kind:      naming.NewKinds(naming.KindCfg, naming.KindSec, naming.KindUsr, naming.KindNscfg),
		Inherit:   keywords.InheritHead,
		Converter: xconfig.NodesConverter,
		Text:      keywords.NewText(fs, "text/kw/core/nodes"),
		Default:   "*",
	},
	{
		Section:   "DEFAULT",
		Option:    "drpnodes",
		Scopable:  true,
		Inherit:   keywords.InheritHead,
		Converter: xconfig.OtherNodesConverter,
		Text:      keywords.NewText(fs, "text/kw/core/drpnodes"),
		Example:   "n1 n2",
	},
	{
		Section:   "DEFAULT",
		Option:    "encapnodes",
		Inherit:   keywords.InheritHead,
		Converter: xconfig.OtherNodesConverter,
		Text:      keywords.NewText(fs, "text/kw/core/encapnodes"),
		Example:   "n1 n2",
	},
	{
		Section:    "DEFAULT",
		Option:     "monitor_action",
		Inherit:    keywords.InheritHead,
		Scopable:   true,
		Candidates: []string{"reboot", "crash", "freezestop", "switch"},
		Text:       keywords.NewText(fs, "text/kw/core/monitor_action"),
		Example:    "reboot",
	},
	{
		Section:  "DEFAULT",
		Option:   "pre_monitor_action",
		Inherit:  keywords.InheritHead,
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/core/pre_monitor_action"),
		Example:  "/bin/true",
	},

	{
		Section: "DEFAULT",
		Option:  "app",
		Default: "default",
		Text:    keywords.NewText(fs, "text/kw/core/app"),
	},
	{
		Section:     "DEFAULT",
		Option:      "env",
		Aliases:     []string{"service_type"},
		Inherit:     keywords.InheritHead,
		Candidates:  rawconfig.Envs,
		DefaultText: keywords.NewText(fs, "text/kw/core/env.default"),
		Text:        keywords.NewText(fs, "text/kw/core/env"),
	},
	{
		Section:   "DEFAULT",
		Option:    "stonith",
		Inherit:   keywords.InheritHead,
		Converter: converters.Bool,
		Default:   "false",
		Depends:   keyop.ParseList("topology=failover"),
		Text:      keywords.NewText(fs, "text/kw/core/stonith"),
	},
	{
		Section:    "DEFAULT",
		Option:     "placement",
		Scopable:   false,
		Inherit:    keywords.InheritHead,
		Default:    "nodes order",
		Candidates: placement.PolicyNames(),
		Text:       keywords.NewText(fs, "text/kw/core/placement"),
	},
	{
		Section:    "DEFAULT",
		Option:     "topology",
		Scopable:   false,
		Default:    "failover",
		Inherit:    keywords.InheritHead,
		Aliases:    []string{"cluster_type"},
		Candidates: []string{"failover", "flex"},
		Text:       keywords.NewText(fs, "text/kw/core/topology"),
	},
	{
		Section:     "DEFAULT",
		Option:      "flex_primary",
		Scopable:    true,
		Inherit:     keywords.InheritHead,
		Converter:   converters.ListLowercase,
		Depends:     keyop.ParseList("topology=flex"),
		DefaultText: keywords.NewText(fs, "text/kw/core/flex_primary.default"),
		Text:        keywords.NewText(fs, "text/kw/core/flex_primary"),
	},
	{
		Section:   "DEFAULT",
		Option:    "shared",
		Scopable:  true,
		Default:   "true",
		Converter: converters.Bool,
		Text:      keywords.NewText(fs, "text/kw/core/shared"),
	},
	{
		Section:   "DEFAULT",
		Option:    "flex_min",
		Aliases:   []string{"flex_min_nodes"},
		Default:   "1",
		Inherit:   keywords.InheritHead,
		Converter: converters.Int,
		Depends:   keyop.ParseList("topology=flex"),
		Text:      keywords.NewText(fs, "text/kw/core/flex_min"),
	},
	{
		Section:     "DEFAULT",
		Option:      "flex_max",
		Aliases:     []string{"flex_max_nodes"},
		Inherit:     keywords.InheritHead,
		Converter:   converters.Int,
		Depends:     keyop.ParseList("topology=flex"),
		Default:     "{flex_target}",
		DefaultText: keywords.NewText(fs, "text/kw/core/flex_max.default"),
		Text:        keywords.NewText(fs, "text/kw/core/flex_max"),
	},
	{
		Section:     "DEFAULT",
		Option:      "flex_target",
		Inherit:     keywords.InheritHead,
		Converter:   converters.Int,
		Depends:     keyop.ParseList("topology=flex"),
		Default:     "1",
		DefaultText: keywords.NewText(fs, "text/kw/core/flex_target.default"),
		Text:        keywords.NewText(fs, "text/kw/core/flex_target"),
	},
	{
		Section:   "DEFAULT",
		Option:    "parents",
		Inherit:   keywords.InheritHead,
		Converter: converters.ListLowercase,
		Text:      keywords.NewText(fs, "text/kw/core/parents"),
	},
	{
		Section:   "DEFAULT",
		Option:    "children",
		Inherit:   keywords.InheritHead,
		Converter: converters.ListLowercase,
		Text:      keywords.NewText(fs, "text/kw/core/children"),
	},
	{
		Section:   "DEFAULT",
		Option:    "slaves",
		Default:   "",
		Inherit:   keywords.InheritHead,
		Converter: converters.ListLowercase,
		Text:      keywords.NewText(fs, "text/kw/core/slaves"),
	},
	{
		Section:    "DEFAULT",
		Option:     "orchestrate",
		Inherit:    keywords.InheritHead,
		Default:    "no",
		Candidates: []string{"no", "ha", "start"},
		Text:       keywords.NewText(fs, "text/kw/core/orchestrate"),
	},
	{
		Section:   "DEFAULT",
		Option:    "priority",
		Default:   fmt.Sprint(priority.Default),
		Scopable:  false,
		Inherit:   keywords.InheritHead,
		Converter: converters.Int,
		Text:      keywords.NewText(fs, "text/kw/core/priority"),
	},
	{
		Section:   "subset",
		Option:    "parallel",
		Kind:      naming.NewKinds(naming.KindSvc, naming.KindVol),
		Scopable:  true,
		Converter: converters.Bool,
		Text:      keywords.NewText(fs, "text/kw/core/parallel"),
	},

	// Secrets
	{
		Section:  "DEFAULT",
		Option:   "cn",
		Scopable: true,
		Example:  "test.opensvc.com",
		Kind:     naming.NewKinds(naming.KindSec),
		Text:     keywords.NewText(fs, "text/kw/core/cn"),
	},
	{
		Section:  "DEFAULT",
		Option:   "c",
		Scopable: true,
		Example:  "FR",
		Kind:     naming.NewKinds(naming.KindSec),
		Text:     keywords.NewText(fs, "text/kw/core/c"),
	},
	{
		Section:  "DEFAULT",
		Option:   "st",
		Scopable: true,
		Example:  "Oise",
		Kind:     naming.NewKinds(naming.KindSec),
		Text:     keywords.NewText(fs, "text/kw/core/st"),
	},
	{
		Section:  "DEFAULT",
		Option:   "l",
		Scopable: true,
		Example:  "Gouvieux",
		Kind:     naming.NewKinds(naming.KindSec),
		Text:     keywords.NewText(fs, "text/kw/core/l"),
	},
	{
		Section:  "DEFAULT",
		Option:   "o",
		Scopable: true,
		Example:  "OpenSVC",
		Kind:     naming.NewKinds(naming.KindSec),
		Text:     keywords.NewText(fs, "text/kw/core/o"),
	},
	{
		Section:  "DEFAULT",
		Option:   "ou",
		Scopable: true,
		Example:  "Lab",
		Kind:     naming.NewKinds(naming.KindSec),
		Text:     keywords.NewText(fs, "text/kw/core/ou"),
	},
	{
		Section:  "DEFAULT",
		Option:   "email",
		Scopable: true,
		Example:  "test@opensvc.com",
		Kind:     naming.NewKinds(naming.KindSec),
		Text:     keywords.NewText(fs, "text/kw/core/email"),
	},
	{
		Section:   "DEFAULT",
		Option:    "alt_names",
		Converter: converters.List,
		Scopable:  true,
		Example:   "www.opensvc.com opensvc.com",
		Kind:      naming.NewKinds(naming.KindSec),
		Text:      keywords.NewText(fs, "text/kw/core/alt_names"),
	},
	{
		Section:   "DEFAULT",
		Option:    "bits",
		Converter: converters.Size,
		Scopable:  true,
		Default:   "4kib",
		Example:   "8192",
		Kind:      naming.NewKinds(naming.KindSec),
		Text:      keywords.NewText(fs, "text/kw/core/bits"),
	},

	// Usr
	{
		Section:   "DEFAULT",
		Option:    "grant",
		Scopable:  true,
		Kind:      naming.NewKinds(naming.KindUsr),
		Inherit:   keywords.InheritHead,
		Converter: converters.ListLowercase,
		Text:      keywords.NewText(fs, "text/kw/core/grant"),
		Example:   "admin:test* guest:*",
	},
	{
		Section:   "DEFAULT",
		Option:    "rollback",
		Scopable:  true,
		Default:   "true",
		Converter: converters.Bool,
		Text:      keywords.NewText(fs, "text/kw/core/rollback"),
	},
	{
		Section:   "DEFAULT",
		Option:    "validity",
		Converter: converters.Duration,
		Scopable:  true,
		Default:   "1y",
		Example:   "10y",
		Kind:      naming.NewKinds(naming.KindSec),
		Text:      keywords.NewText(fs, "text/kw/core/validity"),
	},
	{
		Section:  "DEFAULT",
		Option:   "ca",
		Scopable: true,
		Example:  "ca",
		Kind:     naming.NewKinds(naming.KindSec),
		Text:     keywords.NewText(fs, "text/kw/core/ca"),
	},
	{
		Section:  "DEFAULT",
		Option:   "monitor_schedule",
		Scopable: true,
		Default:  "@5m",
		Text:     keywords.NewText(fs, "text/kw/core/monitor_schedule"),
	},
	{
		Section:  "DEFAULT",
		Option:   "resinfo_schedule",
		Scopable: true,
		Default:  "@60m",
		Text:     keywords.NewText(fs, "text/kw/core/resinfo_schedule"),
	},
	{
		Section:  "DEFAULT",
		Option:   "status_schedule",
		Scopable: true,
		Default:  "@10m",
		Text:     keywords.NewText(fs, "text/kw/core/status_schedule"),
	},
	{
		Section:  "DEFAULT",
		Option:   "comp_schedule",
		Scopable: true,
		Default:  "~00:00-06:00",
		Text:     keywords.NewText(fs, "text/kw/core/comp_schedule"),
	},
	{
		Section:  "DEFAULT",
		Option:   "sync_schedule",
		Scopable: true,
		Default:  "04:00-06:00",
		Text:     keywords.NewText(fs, "text/kw/core/sync_schedule"),
	},
	{
		Section:  "DEFAULT",
		Option:   "run_schedule",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/core/run_schedule"),
	},
	{
		Option:    "timeout",
		Attr:      "Timeout",
		Converter: converters.Duration,
		Scopable:  true,
		Default:   "1h",
		Example:   "2h",
		Text:      keywords.NewText(fs, "text/kw/core/timeout"),
	},
	{
		Option:    "start_timeout",
		Attr:      "StartTimeout",
		Converter: converters.Duration,
		Scopable:  true,
		Example:   "1m30s",
		Text:      keywords.NewText(fs, "text/kw/core/start_timeout"),
	},
	{
		Option:    "stop_timeout",
		Attr:      "StopTimeout",
		Converter: converters.Duration,
		Scopable:  true,
		Example:   "1m30s",
		Text:      keywords.NewText(fs, "text/kw/core/stop_timeout"),
	},
	{
		Option:    "provision_timeout",
		Attr:      "ProvisionTimeout",
		Converter: converters.Duration,
		Scopable:  true,
		Example:   "1m30s",
		Text:      keywords.NewText(fs, "text/kw/core/provision_timeout"),
	},
	{
		Option:    "unprovision_timeout",
		Attr:      "UnprovisionTimeout",
		Converter: converters.Duration,
		Scopable:  true,
		Example:   "1m30s",
		Text:      keywords.NewText(fs, "text/kw/core/unprovision_timeout"),
	},
	{
		Option:    "sync_timeout",
		Attr:      "SyncTimeout",
		Converter: converters.Duration,
		Scopable:  true,
		Example:   "1m30s",
		Text:      keywords.NewText(fs, "text/kw/core/sync_timeout"),
	},
	{
		Option:     "access",
		Attr:       "Access",
		Kind:       naming.NewKinds(naming.KindVol),
		Inherit:    keywords.InheritHead,
		Default:    "rwo",
		Candidates: []string{"rwo", "roo", "rwx", "rox"},
		Scopable:   true,
		Text:       keywords.NewText(fs, "text/kw/core/access"),
	},
	{
		Option:    "size",
		Attr:      "Size",
		Inherit:   keywords.InheritHead,
		Kind:      naming.NewKinds(naming.KindVol),
		Scopable:  true,
		Converter: converters.Size,
		Text:      keywords.NewText(fs, "text/kw/core/size"),
	},
	{
		Option:   "pool",
		Attr:     "Pool",
		Inherit:  keywords.InheritHead,
		Kind:     naming.NewKinds(naming.KindVol),
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/core/pool"),
	},
	{
		Option: "type",
		Text:   keywords.NewText(fs, "text/kw/core/type"),
	},
}

func driverIDFromRID(t Configurer, section string) (driver.ID, error) {
	sectionTypeKey := key.T{
		Section: section,
		Option:  "type",
	}
	sectionType := t.Config().Get(sectionTypeKey)
	rid, err := resourceid.Parse(section)
	if err != nil {
		return driver.ID{}, err
	}
	did := driver.ID{
		Group: rid.DriverGroup(),
		Name:  sectionType,
	}
	return did, nil
}

func keywordLookup(store keywords.Store, k key.T, kind naming.Kind, sectionType string) keywords.Keyword {
	switch k.Section {
	case "data", "env":
		return keywords.Keyword{
			Option:   "*", // trick IsZero()
			Scopable: true,
			Required: false,
		}
	}
	driverGroup := driver.GroupUnknown
	rid, err := resourceid.Parse(k.Section)
	if err == nil {
		driverGroup = rid.DriverGroup()
	}

	if kw := store.Lookup(k, kind, sectionType); !kw.IsZero() {
		// base keyword
		return kw
	}

	for _, i := range driver.ListWithGroup(driverGroup) {
		allocator, ok := i.(func() resource.Driver)
		if !ok {
			continue
		}
		kws := allocator().Manifest().Keywords()
		if kws == nil {
			continue
		}
		store := keywords.Store(kws)
		if kw := store.Lookup(k, kind, sectionType); !kw.IsZero() {
			return kw
		}
	}
	return keywords.Keyword{}
}
