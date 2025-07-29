package object

import (
	"embed"
	"fmt"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/placement"
	"github.com/opensvc/om3/core/priority"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/resourceid"
	"github.com/opensvc/om3/util/key"
)

//go:embed text
var fs embed.FS

var keywordStore = keywords.Store{
	{
		Aliases:   []string{"affinity"},
		Converter: "listlowercase",
		Example:   "svc1 svc2",
		Inherit:   keywords.InheritHead,
		Kind:      naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:    "hard_affinity",
		Section:   "DEFAULT",
		Text:      keywords.NewText(fs, "text/kw/core/hard_affinity"),
	},
	{
		Aliases:   []string{"anti_affinity"},
		Converter: "listlowercase",
		Example:   "svc1 svc2",
		Inherit:   keywords.InheritHead,
		Kind:      naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:    "hard_anti_affinity",
		Section:   "DEFAULT",
		Text:      keywords.NewText(fs, "text/kw/core/hard_anti_affinity"),
	},
	{
		Converter: "listlowercase",
		Example:   "svc1 svc2",
		Inherit:   keywords.InheritHead,
		Kind:      naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:    "soft_affinity",
		Section:   "DEFAULT",
		Text:      keywords.NewText(fs, "text/kw/core/soft_affinity"),
	},
	{
		Converter: "listlowercase",
		Example:   "svc1 svc2",
		Inherit:   keywords.InheritHead,
		Kind:      naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:    "soft_anti_affinity",
		Section:   "DEFAULT",
		Text:      keywords.NewText(fs, "text/kw/core/soft_anti_affinity"),
	},
	{
		DefaultText: keywords.NewText(fs, "text/kw/core/id.default"),
		Option:      "id",
		Section:     "DEFAULT",
		Scopable:    false,
		Text:        keywords.NewText(fs, "text/kw/core/id"),
	},
	{
		Option: "comment",
		Text:   keywords.NewText(fs, "text/kw/core/comment"),
	},
	{
		Converter: "bool",
		Kind:      naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:    "disable",
		Scopable:  true,
		Section:   "DEFAULT",
		Text:      keywords.NewText(fs, "text/kw/core/disable"),
	},
	{
		Converter: "bool",
		Default:   "true",
		Kind:      naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:    "create_pg",
		Scopable:  true,
		Section:   "DEFAULT",
		Text:      keywords.NewText(fs, "text/kw/core/create_pg"),
	},
	{
		Attr:     "PG.Cpus",
		Depends:  keyop.ParseList("create_pg=true"),
		Example:  "0-2",
		Inherit:  keywords.InheritLeaf,
		Kind:     naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:   "pg_cpus",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/core/pg_cpus"),
	},
	{
		Attr:     "PG.Mems",
		Example:  "0-2",
		Inherit:  keywords.InheritLeaf,
		Kind:     naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:   "pg_mems",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/core/pg_mems"),
	},
	{
		Attr:      "PG.CpuShares",
		Converter: "size",
		Example:   "512",
		Inherit:   keywords.InheritLeaf,
		Kind:      naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:    "pg_cpu_shares",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/core/pg_cpu_shares"),
	},
	{
		Attr:     "PG.CpuQuota",
		Example:  "50%@all",
		Inherit:  keywords.InheritLeaf,
		Kind:     naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:   "pg_cpu_quota",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/core/pg_cpu_shares"),
	},
	{
		Attr:     "PG.MemOOMControl",
		Example:  "1",
		Inherit:  keywords.InheritLeaf,
		Kind:     naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:   "pg_mem_oom_control",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/core/pg_mem_oom_control"),
	},
	{
		Attr:      "PG.MemLimit",
		Converter: "size",
		Example:   "512m",
		Inherit:   keywords.InheritLeaf,
		Kind:      naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:    "pg_mem_limit",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/core/pg_mem_limit"),
	},
	{
		Attr:      "PG.VMemLimit",
		Converter: "size",
		Example:   "1g",
		Inherit:   keywords.InheritLeaf,
		Kind:      naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:    "pg_vmem_limit",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/core/pg_vmem_limit"),
	},
	{
		Attr:     "PG.MemSwappiness",
		Example:  "40",
		Inherit:  keywords.InheritLeaf,
		Kind:     naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:   "pg_mem_swappiness",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/core/pg_mem_swappiness"),
	},
	{
		Attr:     "PG.BlkioWeight",
		Example:  "50",
		Inherit:  keywords.InheritLeaf,
		Kind:     naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:   "pg_blkio_weight",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/core/pg_blkio_weight"),
	},
	{
		Converter: "duration",
		Kind:      naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:    "stat_timeout",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/core/stat_timeout"),
	},
	{
		Converter:   "nodes",
		DefaultText: keywords.NewText(fs, "text/kw/core/nodes.default"),
		Example:     "n1 n*",
		Inherit:     keywords.InheritHead,
		Kind:        naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:      "nodes",
		Scopable:    true,
		Section:     "DEFAULT",
		Text:        keywords.NewText(fs, "text/kw/core/nodes"),
	},
	{
		Converter: "nodes",
		Default:   "*",
		Inherit:   keywords.InheritHead,
		Kind:      naming.NewKinds(naming.KindCfg, naming.KindSec, naming.KindUsr, naming.KindNscfg),
		Option:    "nodes",
		Scopable:  true,
		Section:   "DEFAULT",
		Text:      keywords.NewText(fs, "text/kw/core/nodes"),
	},
	{
		Converter: "peers",
		Example:   "n1 n2",
		Inherit:   keywords.InheritHead,
		Option:    "drpnodes",
		Scopable:  true,
		Section:   "DEFAULT",
		Text:      keywords.NewText(fs, "text/kw/core/drpnodes"),
	},
	{
		Converter: "list",
		Example:   "n1 n2",
		Inherit:   keywords.InheritHead,
		Kind:      naming.NewKinds(naming.KindSvc),
		Option:    "encapnodes",
		Section:   "DEFAULT",
		Text:      keywords.NewText(fs, "text/kw/core/encapnodes"),
	},
	{
		Candidates: []string{
			string(instance.MonitorActionCrash),
			string(instance.MonitorActionFreezeStop),
			string(instance.MonitorActionNone),
			string(instance.MonitorActionReboot),
			string(instance.MonitorActionSwitch),
		},
		Converter: "list",
		Default:   string(instance.MonitorActionNone),
		Kind:      naming.NewKinds(naming.KindSvc, naming.KindVol),
		Example:   string(instance.MonitorActionReboot),
		Inherit:   keywords.InheritHead,
		Option:    "monitor_action",
		Scopable:  true,
		Section:   "DEFAULT",
		Text:      keywords.NewText(fs, "text/kw/core/monitor_action"),
	},
	{
		Example:  "/bin/true",
		Inherit:  keywords.InheritHead,
		Kind:     naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:   "pre_monitor_action",
		Scopable: true,
		Section:  "DEFAULT",
		Text:     keywords.NewText(fs, "text/kw/core/pre_monitor_action"),
	},

	{
		Default: "default",
		Option:  "app",
		Section: "DEFAULT",
		Text:    keywords.NewText(fs, "text/kw/core/app"),
	},
	{
		Aliases:     []string{"service_type"},
		DefaultText: keywords.NewText(fs, "text/kw/core/env.default"),
		Inherit:     keywords.InheritHead,
		Option:      "env",
		Section:     "DEFAULT",
		Text:        keywords.NewText(fs, "text/kw/core/env"),
	},
	{
		Converter: "bool",
		Default:   "false",
		Depends:   keyop.ParseList("topology=failover"),
		Inherit:   keywords.InheritHead,
		Kind:      naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:    "stonith",
		Section:   "DEFAULT",
		Text:      keywords.NewText(fs, "text/kw/core/stonith"),
	},
	{
		Candidates: placement.PolicyNames(),
		Default:    "nodes order",
		Inherit:    keywords.InheritHead,
		Kind:       naming.NewKinds(naming.KindSvc),
		Option:     "placement",
		Scopable:   false,
		Section:    "DEFAULT",
		Text:       keywords.NewText(fs, "text/kw/core/placement"),
	},
	{
		Aliases:    []string{"cluster_type"},
		Candidates: []string{"failover", "flex"},
		Default:    "failover",
		Inherit:    keywords.InheritHead,
		Kind:       naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:     "topology",
		Scopable:   false,
		Section:    "DEFAULT",
		Text:       keywords.NewText(fs, "text/kw/core/topology"),
	},
	{
		Converter:   "listlowercase",
		DefaultText: keywords.NewText(fs, "text/kw/core/flex_primary.default"),
		Depends:     keyop.ParseList("topology=flex"),
		Inherit:     keywords.InheritHead,
		Kind:        naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:      "flex_primary",
		Scopable:    true,
		Section:     "DEFAULT",
		Text:        keywords.NewText(fs, "text/kw/core/flex_primary"),
	},
	{
		Converter: "bool",
		Default:   "true",
		Kind:      naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:    "shared",
		Scopable:  true,
		Section:   "DEFAULT",
		Text:      keywords.NewText(fs, "text/kw/core/shared"),
	},
	{
		Aliases:   []string{"flex_min_nodes"},
		Converter: "int",
		Default:   "1",
		Depends:   keyop.ParseList("topology=flex"),
		Inherit:   keywords.InheritHead,
		Kind:      naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:    "flex_min",
		Section:   "DEFAULT",
		Text:      keywords.NewText(fs, "text/kw/core/flex_min"),
	},
	{
		Aliases:     []string{"flex_max_nodes"},
		Converter:   "int",
		Default:     "{#nodes}",
		DefaultText: keywords.NewText(fs, "text/kw/core/flex_max.default"),
		Depends:     keyop.ParseList("topology=flex"),
		Inherit:     keywords.InheritHead,
		Kind:        naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:      "flex_max",
		Section:     "DEFAULT",
		Text:        keywords.NewText(fs, "text/kw/core/flex_max"),
	},
	{
		Converter:   "int",
		Default:     "{flex_min}",
		DefaultText: keywords.NewText(fs, "text/kw/core/flex_target.default"),
		Depends:     keyop.ParseList("topology=flex"),
		Inherit:     keywords.InheritHead,
		Kind:        naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:      "flex_target",
		Section:     "DEFAULT",
		Text:        keywords.NewText(fs, "text/kw/core/flex_target"),
	},
	{
		Converter: "listlowercase",
		Inherit:   keywords.InheritHead,
		Kind:      naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:    "parents",
		Section:   "DEFAULT",
		Text:      keywords.NewText(fs, "text/kw/core/parents"),
	},
	{
		Converter: "listlowercase",
		Inherit:   keywords.InheritHead,
		Kind:      naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:    "children",
		Section:   "DEFAULT",
		Text:      keywords.NewText(fs, "text/kw/core/children"),
	},
	{
		Candidates: []string{"no", "ha", "start"},
		Default:    "no",
		Inherit:    keywords.InheritHead,
		Kind:       naming.NewKinds(naming.KindSvc),
		Option:     "orchestrate",
		Section:    "DEFAULT",
		Text:       keywords.NewText(fs, "text/kw/core/orchestrate"),
	},
	{
		Converter: "int",
		Default:   fmt.Sprint(priority.Default),
		Inherit:   keywords.InheritHead,
		Kind:      naming.NewKinds(naming.KindSvc),
		Option:    "priority",
		Scopable:  false,
		Section:   "DEFAULT",
		Text:      keywords.NewText(fs, "text/kw/core/priority"),
	},
	{
		Converter: "bool",
		Kind:      naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:    "parallel",
		Scopable:  true,
		Section:   "subset",
		Text:      keywords.NewText(fs, "text/kw/core/parallel"),
	},

	// DataStores
	{
		Converter: "list",
		Default:   "{namespace}",
		Example:   "ns1 ns2",
		Kind:      naming.NewKinds(naming.KindSec, naming.KindCfg),
		Option:    "share",
		Scopable:  false,
		Section:   "DEFAULT",
		Text:      keywords.NewText(fs, "text/kw/core/share"),
	},

	// Secrets
	{
		Example:  "test.opensvc.com",
		Kind:     naming.NewKinds(naming.KindSec, naming.KindUsr),
		Option:   "cn",
		Scopable: true,
		Section:  "DEFAULT",
		Text:     keywords.NewText(fs, "text/kw/core/cn"),
	},
	{
		Example:  "FR",
		Kind:     naming.NewKinds(naming.KindSec, naming.KindUsr),
		Option:   "c",
		Scopable: true,
		Section:  "DEFAULT",
		Text:     keywords.NewText(fs, "text/kw/core/c"),
	},
	{
		Section:  "DEFAULT",
		Option:   "st",
		Scopable: true,
		Example:  "Oise",
		Kind:     naming.NewKinds(naming.KindSec, naming.KindUsr),
		Text:     keywords.NewText(fs, "text/kw/core/st"),
	},
	{
		Example:  "Gouvieux",
		Kind:     naming.NewKinds(naming.KindSec, naming.KindUsr),
		Option:   "l",
		Scopable: true,
		Section:  "DEFAULT",
		Text:     keywords.NewText(fs, "text/kw/core/l"),
	},
	{
		Example:  "OpenSVC",
		Kind:     naming.NewKinds(naming.KindSec, naming.KindUsr),
		Option:   "o",
		Scopable: true,
		Section:  "DEFAULT",
		Text:     keywords.NewText(fs, "text/kw/core/o"),
	},
	{
		Example:  "Lab",
		Kind:     naming.NewKinds(naming.KindSec, naming.KindUsr),
		Option:   "ou",
		Scopable: true,
		Section:  "DEFAULT",
		Text:     keywords.NewText(fs, "text/kw/core/ou"),
	},
	{
		Example:  "test@opensvc.com",
		Kind:     naming.NewKinds(naming.KindSec, naming.KindUsr),
		Option:   "email",
		Scopable: true,
		Section:  "DEFAULT",
		Text:     keywords.NewText(fs, "text/kw/core/email"),
	},
	{
		Converter: "list",
		Example:   "www.opensvc.com opensvc.com",
		Kind:      naming.NewKinds(naming.KindSec, naming.KindUsr),
		Option:    "alt_names",
		Scopable:  true,
		Section:   "DEFAULT",
		Text:      keywords.NewText(fs, "text/kw/core/alt_names"),
	},
	{
		Converter: "size",
		Default:   "4kib",
		Example:   "8192",
		Kind:      naming.NewKinds(naming.KindSec, naming.KindUsr),
		Option:    "bits",
		Scopable:  true,
		Section:   "DEFAULT",
		Text:      keywords.NewText(fs, "text/kw/core/bits"),
	},

	// Usr
	{
		Converter: "listlowercase",
		Example:   "admin:test* guest:*",
		Inherit:   keywords.InheritHead,
		Kind:      naming.NewKinds(naming.KindUsr),
		Option:    "grant",
		Scopable:  true,
		Section:   "DEFAULT",
		Text:      keywords.NewText(fs, "text/kw/core/grant"),
	},
	{
		Converter: "bool",
		Default:   "true",
		Kind:      naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:    "rollback",
		Scopable:  true,
		Section:   "DEFAULT",
		Text:      keywords.NewText(fs, "text/kw/core/rollback"),
	},
	{
		Converter: "duration",
		Default:   "1y",
		Example:   "10y",
		Kind:      naming.NewKinds(naming.KindSec, naming.KindUsr),
		Option:    "validity",
		Scopable:  true,
		Section:   "DEFAULT",
		Text:      keywords.NewText(fs, "text/kw/core/validity"),
	},
	{
		Example:  "ca",
		Kind:     naming.NewKinds(naming.KindSec, naming.KindUsr),
		Option:   "ca",
		Scopable: true,
		Section:  "DEFAULT",
		Text:     keywords.NewText(fs, "text/kw/core/ca"),
	},
	{
		Default:  "@1m",
		Kind:     naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:   "monitor_schedule",
		Scopable: true,
		Section:  "DEFAULT",
		Text:     keywords.NewText(fs, "text/kw/core/monitor_schedule"),
	},
	{
		Default:  "@60m",
		Kind:     naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:   "resinfo_schedule",
		Scopable: true,
		Section:  "DEFAULT",
		Text:     keywords.NewText(fs, "text/kw/core/resinfo_schedule"),
	},
	{
		Default:  "@10m",
		Kind:     naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:   "status_schedule",
		Scopable: true,
		Section:  "DEFAULT",
		Text:     keywords.NewText(fs, "text/kw/core/status_schedule"),
	},
	{
		Default:  "~00:00-06:00",
		Kind:     naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:   "comp_schedule",
		Scopable: true,
		Section:  "DEFAULT",
		Text:     keywords.NewText(fs, "text/kw/core/comp_schedule"),
	},
	{
		Default:  "04:00-06:00",
		Kind:     naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:   "sync_schedule",
		Scopable: true,
		Section:  "DEFAULT",
		Text:     keywords.NewText(fs, "text/kw/core/sync_schedule"),
	},
	{
		Kind:     naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:   "run_schedule",
		Scopable: true,
		Section:  "DEFAULT",
		Text:     keywords.NewText(fs, "text/kw/core/run_schedule"),
	},
	{
		Attr:      "Timeout",
		Converter: "duration",
		Default:   "1h",
		Example:   "2h",
		Kind:      naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:    "timeout",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/core/timeout"),
	},
	{
		Attr:      "StatusTimeout",
		Converter: "duration",
		Default:   "1m",
		Example:   "10s",
		Kind:      naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:    "status_timeout",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/core/status_timeout"),
	},
	{
		Attr:      "StartTimeout",
		Converter: "duration",
		Example:   "1m30s",
		Kind:      naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:    "start_timeout",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/core/start_timeout"),
	},
	{
		Attr:      "StopTimeout",
		Converter: "duration",
		Example:   "1m30s",
		Kind:      naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:    "stop_timeout",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/core/stop_timeout"),
	},
	{
		Attr:      "Provision",
		Converter: "bool",
		Default:   "true",
		Kind:      naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:    "provision",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/core/provision"),
	},
	{
		Attr:      "Unprovision",
		Converter: "bool",
		Default:   "true",
		Kind:      naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:    "unprovision",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/core/unprovision"),
	},
	{
		Attr:      "ProvisionTimeout",
		Converter: "duration",
		Example:   "1m30s",
		Kind:      naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:    "provision_timeout",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/core/provision_timeout"),
	},
	{
		Attr:      "UnprovisionTimeout",
		Converter: "duration",
		Example:   "1m30s",
		Kind:      naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:    "unprovision_timeout",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/core/unprovision_timeout"),
	},
	{
		Attr:      "SyncTimeout",
		Converter: "duration",
		Example:   "1m30s",
		Kind:      naming.NewKinds(naming.KindSvc, naming.KindVol),
		Option:    "sync_timeout",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/core/sync_timeout"),
	},
	{
		Attr:       "Access",
		Candidates: []string{"rwo", "roo", "rwx", "rox"},
		Default:    "rwo",
		Inherit:    keywords.InheritHead,
		Kind:       naming.NewKinds(naming.KindVol),
		Option:     "access",
		Scopable:   true,
		Text:       keywords.NewText(fs, "text/kw/core/access"),
	},
	{
		Attr:      "DevicesFrom",
		Inherit:   keywords.InheritHead,
		Kind:      naming.NewKinds(naming.KindVol),
		Option:    "devices_from",
		Converter: "list",
		Text:      keywords.NewText(fs, "text/kw/core/devices_from"),
	},
	{
		Attr:      "Size",
		Converter: "size",
		Inherit:   keywords.InheritHead,
		Kind:      naming.NewKinds(naming.KindVol),
		Option:    "size",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/core/size"),
	},
	{
		Attr:     "Pool",
		Inherit:  keywords.InheritHead,
		Kind:     naming.NewKinds(naming.KindVol),
		Option:   "pool",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/core/pool"),
	},
	{
		Kind:   naming.NewKinds(naming.KindSvc, naming.KindVol),
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
	case "data", "env", "labels":
		return keywords.Keyword{
			Option:   "*", // trick IsZero()
			Scopable: kind != naming.KindInvalid,
			Inherit:  keywords.InheritLeaf,
		}
	}
	driverGroup := driver.GroupUnknown
	rid, err := resourceid.Parse(k.Section)
	if err == nil {
		driverGroup = rid.DriverGroup()
	}

	if driverGroup == driver.GroupVolume {
		sectionType = ""
	}

	// driver keyword
	var drivers driver.Registry
	if k.Section == "*" && driverGroup == driver.GroupUnknown {
		drivers = driver.All
	} else if sectionType != "" {
		drivers = driver.All.WithID(driverGroup, sectionType)
	} else {
		drivers = driver.All.WithGroup(driverGroup)
	}

	for _, i := range drivers {
		allocator, ok := i.(func() resource.Driver)
		if !ok {
			continue
		}
		kws := allocator().Manifest().Keywords()
		if kws == nil {
			continue
		}
		if kw := keywords.Store(kws).Lookup(k, kind, sectionType); !kw.IsZero() {
			return kw
		}
	}

	// base keyword
	if kw := store.Lookup(k, kind, sectionType); !kw.IsZero() {
		return kw
	}

	return keywords.Keyword{}
}

// KeywordStoreWithDrivers return the keywords supported by a specific
// object kind, including all resource drivers keywords.
func KeywordStoreWithDrivers(kind naming.Kind) keywords.Store {
	var store keywords.Store
	if kind == naming.KindCcfg {
		store = append(store, ccfgKeywordStore...)
	} else {
		store = append(store, keywordStore...)
	}
	for _, drvID := range driver.List() {
		factory := resource.NewResourceFunc(drvID)
		if factory == nil {
			// node drivers don't have a factory, skip them
			continue
		}
		r := factory()
		manifest := r.Manifest()
		if !manifest.Kinds.Has(kind) {
			continue
		}
		for _, kw := range manifest.Keywords() {
			kw.Section = drvID.Group.String()
			kw.Types = []string{drvID.Name}
			store = append(store, kw)
		}
	}
	return store
}
