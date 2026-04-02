package object

import (
	"fmt"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/kwoption"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/daemonenv"
	"github.com/opensvc/om3/v3/util/key"
)

const (
	DefaultNodeMaxParallel = 10
)

var (
	kwNodeOCI = keywords.Keyword{
		Option:  "oci",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.oci"),
	}
	kwNodeUUID = keywords.Keyword{
		Option:  "uuid",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.uuid"),
	}
	kwNodePRKey = keywords.Keyword{
		DefaultText: keywords.NewText(fs, "text/kw/node/node.prkey.default"),
		Option:      "prkey",
		Section:     "node",
		Text:        keywords.NewText(fs, "text/kw/node/node.prkey"),
	}
	kwNodeConnectTo = keywords.Keyword{
		Example: "1.2.3.4",
		Option:  "connect_to",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.connect_to"),
	}
	kwNodeMemBytes = keywords.Keyword{
		Converter: "size",
		Example:   "256mb",
		Option:    "mem_bytes",
		Section:   "node",
		Text:      keywords.NewText(fs, "text/kw/node/node.mem_bytes"),
	}
	kwNodeMemBanks = keywords.Keyword{
		Converter: "int",
		Example:   "4",
		Option:    "mem_banks",
		Section:   "node",
		Text:      keywords.NewText(fs, "text/kw/node/node.mem_banks"),
	}
	kwNodeMemSlots = keywords.Keyword{
		Converter: "int",
		Example:   "4",
		Option:    "mem_slots",
		Section:   "node",
		Text:      keywords.NewText(fs, "text/kw/node/node.mem_slots"),
	}
	kwNodeOSVendor = keywords.Keyword{
		Example: "Digital",
		Option:  "os_vendor",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.os_vendor"),
	}
	kwNodeOSRelease = keywords.Keyword{
		Example: "5",
		Option:  "os_release",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.os_release"),
	}
	kwNodeOSKernel = keywords.Keyword{
		Example: "5.1234",
		Option:  "os_kernel",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.os_kernel"),
	}
	kwNodeOSArch = keywords.Keyword{
		Example: "5.1234",
		Option:  "os_arch",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.os_arch"),
	}
	kwNodeCPUFreq = keywords.Keyword{
		Example: "3.2 Ghz",
		Option:  "cpu_freq",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.cpu_freq"),
	}
	kwNodeCPUThreads = keywords.Keyword{
		Converter: "int",
		Example:   "4",
		Option:    "cpu_threads",
		Section:   "node",
		Text:      keywords.NewText(fs, "text/kw/node/node.cpu_threads"),
	}
	kwNodeCPUCores = keywords.Keyword{
		Converter: "int",
		Example:   "2",
		Option:    "cpu_cores",
		Section:   "node",
		Text:      keywords.NewText(fs, "text/kw/node/node.cpu_cores"),
	}
	kwNodeCPUDies = keywords.Keyword{
		Converter: "int",
		Example:   "1",
		Option:    "cpu_dies",
		Section:   "node",
		Text:      keywords.NewText(fs, "text/kw/node/node.cpu_dies"),
	}
	kwNodeCPUModel = keywords.Keyword{
		Example: "Alpha EV5",
		Option:  "cpu_model",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.cpu_model"),
	}
	kwNodeSerial = keywords.Keyword{
		Example: "abcdef0123456",
		Option:  "serial",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.serial"),
	}
	kwNodeBIOSVersion = keywords.Keyword{
		Example: "1.025",
		Option:  "bios_version",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.bios_version"),
	}
	kwNodeSPVersion = keywords.Keyword{
		Example: "1.026",
		Option:  "sp_version",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.sp_version"),
	}
	kwNodeEnclosure = keywords.Keyword{
		Example: "1",
		Option:  "enclosure",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.enclosure"),
	}
	kwNodeTZ = keywords.Keyword{
		Example: "+0200",
		Option:  "tz",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.tz"),
	}
	kwNodeManufacturer = keywords.Keyword{
		Example: "Digital",
		Option:  "manufacturer",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.manufacturer"),
	}
	kwNodeModel = keywords.Keyword{
		Example: "ds20e",
		Option:  "model",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.model"),
	}
	kwNodeArraySchedule = keywords.Keyword{
		Option:  "schedule",
		Section: "array",
		Text:    keywords.NewText(fs, "text/kw/node/array.schedule"),
	}
	kwNodeArrayXtremioName = keywords.Keyword{
		Example: "array1",
		Option:  "name",
		Section: "array",
		Text:    keywords.NewText(fs, "text/kw/node/array.xtremio.name"),
		Types:   []string{"xtremio"},
	}
	kwNodeBackupSchedule = keywords.Keyword{
		Option:  "schedule",
		Section: "backup",
		Text:    keywords.NewText(fs, "text/kw/node/backup.schedule"),
	}
	kwNodeSwitchSchedule = keywords.Keyword{
		Option:  "schedule",
		Section: "switch",
		Text:    keywords.NewText(fs, "text/kw/node/switch.schedule"),
	}
	nodePrivateKeywords = []*keywords.Keyword{
		&kwNodeOCI,
		&kwNodeUUID,
		&kwNodePRKey,
		&kwNodeConnectTo,
		&kwNodeMemBytes,
		&kwNodeMemBanks,
		&kwNodeMemSlots,
		&kwNodeOSVendor,
		&kwNodeOSRelease,
		&kwNodeOSKernel,
		&kwNodeOSArch,
		&kwNodeCPUFreq,
		&kwNodeCPUThreads,
		&kwNodeCPUDies,
		&kwNodeCPUModel,
		&kwNodeSerial,
		&kwNodeBIOSVersion,
		&kwNodeSPVersion,
		&kwNodeEnclosure,
		&kwNodeTZ,
		&kwNodeManufacturer,
		&kwNodeModel,
		&kwNodeBackupSchedule,
		&kwNodeSwitchSchedule,
	}

	kwNodeSecureFetch = keywords.Keyword{
		Converter: "bool",
		Default:   "true",
		Option:    "secure_fetch",
		Section:   "node",
		Text:      keywords.NewText(fs, "text/kw/node/node.secure_fetch"),
	}
	kwNodeMinAvailMemPct = keywords.Keyword{
		Aliases:   []string{"min_avail_mem"},
		Converter: "int",
		Default:   "2",
		Option:    "min_avail_mem_pct",
		Section:   "node",
		Text:      keywords.NewText(fs, "text/kw/node/node.min_avail_mem_pct"),
	}
	kwNodeMinAvailSwapPct = keywords.Keyword{
		Aliases:   []string{"min_avail_swap"},
		Converter: "int",
		Default:   "10",
		Option:    "min_avail_swap_pct",
		Section:   "node",
		Text:      keywords.NewText(fs, "text/kw/node/node.min_avail_swap_pct"),
	}
	kwNodeEnv = keywords.Keyword{
		Default: "TST",
		Option:  "env",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.env"),
	}
	kwNodeMaxGreetTimeout = keywords.Keyword{
		Converter: "duration",
		Option:    "max_greet_timeout",
		Section:   "console",
		Default:   "20s",
		Text:      keywords.NewText(fs, "text/kw/node/console.max_greet_timeout"),
	}
	kwNodeConsoleMaxSeats = keywords.Keyword{
		Converter: "int",
		Option:    "max_seats",
		Section:   "console",
		Default:   "1",
		Text:      keywords.NewText(fs, "text/kw/node/console.max_seats"),
	}
	kwNodeConsoleInsecure = keywords.Keyword{
		Converter: "bool",
		Option:    "insecure",
		Section:   "console",
		Text:      keywords.NewText(fs, "text/kw/node/console.insecure"),
	}
	kwNodeConsoleServer = keywords.Keyword{
		Option:  "server",
		Section: "console",
		Text:    keywords.NewText(fs, "text/kw/node/console.server"),
	}
	kwNodeMaxParallel = keywords.Keyword{
		Converter: "int",
		Default:   fmt.Sprintf("%d", DefaultNodeMaxParallel),
		Option:    "max_parallel",
		Section:   "node",
		Text:      keywords.NewText(fs, "text/kw/node/node.max_parallel"),
	}
	kwNodeMaxKeySize = keywords.Keyword{
		Converter: "size",
		Default:   "1mb",
		Option:    "max_key_size",
		Section:   "node",
		Text:      keywords.NewText(fs, "text/kw/node/node.max_key_size"),
	}
	kwNodeAllowedNetworks = keywords.Keyword{
		Converter: "list",
		Default:   "10.0.0.0/8 172.16.0.0/24 192.168.0.0/16",
		Option:    "allowed_networks",
		Section:   "node",
		Text:      keywords.NewText(fs, "text/kw/node/node.allowed_networks"),
	}
	kwNodeLocCountry = keywords.Keyword{
		Example: "fr",
		Option:  "loc_country",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.loc_country"),
	}
	kwNodeLocCity = keywords.Keyword{
		Example: "Paris",
		Option:  "loc_city",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.loc_city"),
	}
	kwNodeLocZIP = keywords.Keyword{
		Example: "75017",
		Option:  "loc_zip",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.loc_zip"),
	}
	kwNodeLocAddr = keywords.Keyword{
		Example: "7 rue blanche",
		Option:  "loc_addr",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.loc_addr"),
	}
	kwNodeLocBuilding = keywords.Keyword{
		Example: "Crystal",
		Option:  "loc_building",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.loc_building"),
	}
	kwNodeLocFloor = keywords.Keyword{
		Example: "21",
		Option:  "loc_floor",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.loc_floor"),
	}
	kwNodeLocRoom = keywords.Keyword{
		Example: "102",
		Option:  "loc_room",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.loc_room"),
	}
	kwNodeLocRack = keywords.Keyword{
		Example: "R42",
		Option:  "loc_rack",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.loc_rack"),
	}
	kwNodeSecZone = keywords.Keyword{
		Example: "dmz1",
		Option:  "sec_zone",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.sec_zone"),
	}
	kwNodeTeamInteg = keywords.Keyword{
		Example: "TINT",
		Option:  "team_integ",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.team_integ"),
	}
	kwNodeTeamSupport = keywords.Keyword{
		Example: "TSUP",
		Option:  "team_support",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.team_support"),
	}
	kwNodeAssetEnv = keywords.Keyword{
		Example: "Production",
		Option:  "asset_env",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.asset_env"),
	}
	kwNodeDBOpensvc = keywords.Keyword{
		Example: "https://collector.opensvc.com",
		Option:  "dbopensvc",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.dbopensvc"),
	}
	kwNodeCollector = keywords.Keyword{
		Example: "https://collector.opensvc.com",
		Option:  "collector",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.collector"),
	}
	kwNodeCollectorServer = keywords.Keyword{
		Example:     "https://collector.opensvc.com/server",
		Option:      "collector_server",
		Section:     "node",
		Text:        keywords.NewText(fs, "text/kw/node/node.collector_server"),
		DefaultText: keywords.NewText(fs, "text/kw/node/node.collector_server.default"),
	}
	kwNodeCollectorFeeder = keywords.Keyword{
		Example:     "https://collector.opensvc.com/feeder",
		Option:      "collector_feeder",
		Section:     "node",
		Text:        keywords.NewText(fs, "text/kw/node/node.collector_feeder"),
		DefaultText: keywords.NewText(fs, "text/kw/node/node.collector_feeder.default"),
	}
	kwNodeCollectorTimeout = keywords.Keyword{
		Option:    "collector_timeout",
		Section:   "node",
		Converter: "duration",
		Default:   "5s",
		Text:      keywords.NewText(fs, "text/kw/node/node.collector_timeout"),
	}
	kwNodeDBInsecure = keywords.Keyword{
		Converter: "bool",
		Option:    "dbinsecure",
		Section:   "node",
		Text:      keywords.NewText(fs, "text/kw/node/node.dbinsecure"),
	}
	kwNodeDBCompliance = keywords.Keyword{
		DefaultText: keywords.NewText(fs, "text/kw/node/node.dbcompliance.default"),
		Example:     "https://collector.opensvc.com",
		Option:      "dbcompliance",
		Section:     "node",
		Text:        keywords.NewText(fs, "text/kw/node/node.dbcompliance"),
	}
	kwNodeDBLog = keywords.Keyword{
		Converter: "bool",
		Default:   "true",
		Option:    "dblog",
		Section:   "node",
		Text:      keywords.NewText(fs, "text/kw/node/node.dblog"),
	}
	kwNodeBranch = keywords.Keyword{
		Example: "1.9",
		Option:  "branch",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.branch"),
	}
	kwNodeRepo = keywords.Keyword{
		Example: "http://opensvc.repo.corp",
		Option:  "repo",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.repo"),
	}
	kwNodeRepoPkg = keywords.Keyword{
		Example: "http://repo.opensvc.com",
		Option:  "repopkg",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.repopkg"),
	}
	kwNodeRepoComp = keywords.Keyword{
		Example: "http://compliance.repo.corp",
		Option:  "repocomp",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.repocomp"),
	}
	kwNodeRUser = keywords.Keyword{
		Default: "root",
		Example: "root opensvc@node1",
		Option:  "ruser",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.ruser"),
	}
	kwNodeMaintenanceGracePeriod = keywords.Keyword{
		Default:   "60",
		Converter: "duration",
		Option:    "maintenance_grace_period",
		Section:   "node",
		Text:      keywords.NewText(fs, "text/kw/node/node.maintenance_grace_period"),
	}
	kwNodeRejoinGracePeriod = keywords.Keyword{
		Converter: "duration",
		Default:   "90s",
		Option:    "rejoin_grace_period",
		Section:   "node",
		Text:      keywords.NewText(fs, "text/kw/node/node.rejoin_grace_period"),
	}
	kwNodeReadyPeriod = keywords.Keyword{
		Converter: "duration",
		Default:   "5s",
		Option:    "ready_period",
		Section:   "node",
		Text:      keywords.NewText(fs, "text/kw/node/node.ready_period"),
	}
	kwNodeDequeueActionSchedule = keywords.Keyword{
		Option:  "schedule",
		Section: "dequeue_actions",
		Text:    keywords.NewText(fs, "text/kw/node/dequeue_actions.schedule"),
	}
	kwNodeSysreportSchedule = keywords.Keyword{
		Default: "~00:00-06:00",
		Option:  "schedule",
		Section: "sysreport",
		Text:    keywords.NewText(fs, "text/kw/node/sysreport.schedule"),
	}
	kwNodeComplianceSchedule = keywords.Keyword{
		Default: "02:00-06:00",
		Option:  "schedule",
		Section: "compliance",
		Text:    keywords.NewText(fs, "text/kw/node/compliance.schedule"),
	}
	kwNodeComplianceAutoUpdate = keywords.Keyword{
		Converter: "bool",
		Default:   "false",
		Option:    "auto_update",
		Section:   "compliance",
		Text:      keywords.NewText(fs, "text/kw/node/compliance.auto_update"),
	}
	kwNodeChecksSchedule = keywords.Keyword{
		Default: "~00:00-06:00",
		Option:  "schedule",
		Section: "checks",
		Text:    keywords.NewText(fs, "text/kw/node/checks.schedule"),
	}
	kwNodePackagesSchedule = keywords.Keyword{
		Default: "~00:00-06:00",
		Option:  "schedule",
		Section: "packages",
		Text:    keywords.NewText(fs, "text/kw/node/packages.schedule"),
	}
	kwNodePatchesSchedule = keywords.Keyword{
		Default: "~00:00-06:00",
		Option:  "schedule",
		Section: "patches",
		Text:    keywords.NewText(fs, "text/kw/node/patches.schedule"),
	}
	kwNodeAssetSchedule = keywords.Keyword{
		Default: "~00:00-06:00",
		Option:  "schedule",
		Section: "asset",
		Text:    keywords.NewText(fs, "text/kw/node/asset.schedule"),
	}
	kwNodeDisksSchedule = keywords.Keyword{
		Default: "~00:00-06:00",
		Option:  "schedule",
		Section: "disks",
		Text:    keywords.NewText(fs, "text/kw/node/disks.schedule"),
	}
	kwNodeListenerCRL = keywords.Keyword{
		Default: rawconfig.Paths.CACRL,
		Example: "https://crl.opensvc.com",
		Option:  "crl",
		Section: "listener",
		Text:    keywords.NewText(fs, "text/kw/node/listener.crl"),
	}
	kwNodeListenerDNSSockUID = keywords.Keyword{
		Default: "953",
		Option:  "dns_sock_uid",
		Section: "listener",
		Text:    keywords.NewText(fs, "text/kw/node/listener.dns_sock_uid"),
	}
	kwNodeListenerDNSSockGID = keywords.Keyword{
		Default: "953",
		Option:  "dns_sock_gid",
		Section: "listener",
		Text:    keywords.NewText(fs, "text/kw/node/listener.dns_sock_gid"),
	}
	kwNodeListenerAddr = keywords.Keyword{
		Aliases:  []string{"tls_addr"},
		Default:  "",
		Example:  "1.2.3.4",
		Option:   "addr",
		Scopable: true,
		Section:  "listener",
		Text:     keywords.NewText(fs, "text/kw/node/listener.addr"),
	}
	kwNodeListenerPort = keywords.Keyword{
		Aliases:   []string{"tls_port"},
		Converter: "int",
		Default:   fmt.Sprintf("%d", daemonenv.HTTPPort),
		Option:    "port",
		Scopable:  true,
		Section:   "listener",
		Text:      keywords.NewText(fs, "text/kw/node/listener.port"),
	}
	kwNodeListenerOpenIDIssuer = keywords.Keyword{
		Example: "https://keycloak.opensvc.com/auth/realms/clusters",
		Option:  "openid_issuer",
		Section: "listener",
		Text:    keywords.NewText(fs, "text/kw/node/listener.openid_issuer"),
	}
	kwNodeListenerOpenIDClientID = keywords.Keyword{
		Default: "om3",
		Option:  "openid_client_id",
		Section: "listener",
		Text:    keywords.NewText(fs, "text/kw/node/listener.openid_client_id"),
	}
	kwNodeListenerRateLimiterRate = keywords.Keyword{
		Default:   "20",
		Converter: "int",
		Option:    "rate_limiter_rate",
		Section:   "listener",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/node/listener.rate_limiter_rate"),
	}
	kwNodeListenerRateLimiterBurst = keywords.Keyword{
		Default:   "100",
		Converter: "int",
		Option:    "rate_limiter_burst",
		Section:   "listener",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/node/listener.rate_limiter_burst"),
	}
	kwNodeListenerRateLimiterExpires = keywords.Keyword{
		Default:   "60s",
		Converter: "duration",
		Option:    "rate_limiter_expires",
		Section:   "listener",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/node/listener.rate_limiter_expires"),
	}
	kwNodeSyslogFacility = keywords.Keyword{
		Default: "daemon",
		Option:  "facility",
		Section: "syslog",
		Text:    keywords.NewText(fs, "text/kw/node/syslog.facility"),
	}
	kwNodeSyslogLevel = keywords.Keyword{
		Candidates: []string{"critical", "error", "warning", "info", "debug"},
		Default:    "info",
		Option:     "level",
		Section:    "syslog",
		Text:       keywords.NewText(fs, "text/kw/node/syslog.level"),
	}
	kwNodeSyslogHost = keywords.Keyword{
		DefaultText: keywords.NewText(fs, "text/kw/node/syslog.host.default"),
		Option:      "host",
		Section:     "syslog",
		Text:        keywords.NewText(fs, "text/kw/node/syslog.host"),
	}
	kwNodeSyslogPort = keywords.Keyword{
		Default: "514",
		Option:  "port",
		Section: "syslog",
		Text:    keywords.NewText(fs, "text/kw/node/syslog.port"),
	}
	kwNodeClusterDNS = keywords.Keyword{
		Converter: "list",
		Option:    "dns",
		Scopable:  true,
		Section:   "cluster",
		Text:      keywords.NewText(fs, "text/kw/node/cluster.dns"),
	}
	kwNodeClusterCA = keywords.Keyword{
		DefaultText: keywords.NewText(fs, "text/kw/node/cluster.ca.default"),
		Converter:   "list",
		Option:      "ca",
		Section:     "cluster",
		Text:        keywords.NewText(fs, "text/kw/node/cluster.ca"),
	}
	kwNodeClusterCert = keywords.Keyword{
		DefaultText: keywords.NewText(fs, "text/kw/node/cluster.cert.default"),
		Option:      "cert",
		Section:     "cluster",
		Text:        keywords.NewText(fs, "text/kw/node/cluster.cert"),
	}
	kwNodeClusterID = keywords.Keyword{
		DefaultText: keywords.NewText(fs, "text/kw/node/cluster.id.default"),
		Option:      "id",
		Scopable:    true,
		Section:     "cluster",
		Text:        keywords.NewText(fs, "text/kw/node/cluster.id"),
	}
	kwNodeClusterName = keywords.Keyword{
		DefaultText: keywords.NewText(fs, "text/kw/node/cluster.name.default"),
		Option:      "name",
		Section:     "cluster",
		Text:        keywords.NewText(fs, "text/kw/node/cluster.name"),
	}
	kwNodeClusterSecret = keywords.Keyword{
		DefaultText: keywords.NewText(fs, "text/kw/node/cluster.secret.default"),
		Option:      "secret",
		Scopable:    true,
		Section:     "cluster",
		Text:        keywords.NewText(fs, "text/kw/node/cluster.secret"),
	}
	kwNodeClusterNodes = keywords.Keyword{
		Converter: "list",
		Option:    "nodes",
		Section:   "cluster",
		Text:      keywords.NewText(fs, "text/kw/node/cluster.nodes"),
	}
	kwNodeClusterDRPNodes = keywords.Keyword{
		Converter: "list",
		Option:    "drpnodes",
		Section:   "cluster",
		Text:      keywords.NewText(fs, "text/kw/node/cluster.drpnodes"),
	}
	kwNodeClusterEnvs = keywords.Keyword{
		Converter: "list",
		Option:    "envs",
		Default:   "CERT DEV DRP FOR INT PRA PRD PRJ PPRD QUAL REC STG TMP TST UAT",
		Section:   "cluster",
		Text:      keywords.NewText(fs, "text/kw/node/cluster.envs"),
	}
	kwNodeClusterQuorum = keywords.Keyword{
		Converter: "bool",
		Default:   "false",
		Option:    "quorum",
		Section:   "cluster",
		Text:      keywords.NewText(fs, "text/kw/node/cluster.quorum"),
	}
	kwNodeSSHKey = keywords.Keyword{
		Default: "opensvc",
		Option:  "sshkey",
		Section: "node",
		Text:    keywords.NewText(fs, "text/kw/node/node.sshkey"),
	}
	kwNodeSplitAction = keywords.Keyword{
		Candidates: []string{"crash", "reboot", "disabled"},
		Default:    "crash",
		Option:     "split_action",
		Scopable:   true,
		Section:    "node",
		Text:       keywords.NewText(fs, "text/kw/node/node.split_action"),
	}
	kwNodeArbitratorURI = keywords.Keyword{
		Aliases:  []string{"name"},
		Example:  "http://www.opensvc.com",
		Option:   "uri",
		Required: true,
		Section:  "arbitrator",
		Text:     keywords.NewText(fs, "text/kw/node/arbitrator.uri"),
	}
	kwNodeArbitratorInsecure = keywords.Keyword{
		Converter: "bool",
		Default:   "false",
		Option:    "insecure",
		Section:   "arbitrator",
		Text:      keywords.NewText(fs, "text/kw/node/arbitrator.insecure"),
	}
	kwNodeArbitratorWeight = keywords.Keyword{
		Converter: "int",
		Default:   "1",
		Option:    "weight",
		Section:   "arbitrator",
		Text:      keywords.NewText(fs, "text/kw/node/arbitrator.weight"),
	}
	kwNodeStonithCommand = keywords.Keyword{
		Converter: "shlex",
		Example:   "/bin/true",
		Option:    "command",
		Aliases:   []string{"cmd"},
		Required:  true,
		Scopable:  true,
		Section:   "stonith",
		Text:      keywords.NewText(fs, "text/kw/node/stonith.command"),
	}
	kwNodeHBType = keywords.Keyword{
		Candidates: []string{"unicast", "multicast", "disk", "relay"},
		Option:     "type",
		Required:   true,
		Section:    "hb",
		Text:       keywords.NewText(fs, "text/kw/node/hb.type"),
	}
	kwNodeHBUnicastAddr = keywords.Keyword{
		DefaultText: keywords.NewText(fs, "text/kw/node/hb.unicast.addr.default"),
		Example:     "1.2.3.4",
		Option:      "addr",
		Scopable:    true,
		Section:     "hb",
		Text:        keywords.NewText(fs, "text/kw/node/hb.unicast.addr"),
		Types:       []string{"unicast"},
	}
	kwNodeHBUnicastIntf = keywords.Keyword{
		DefaultText: keywords.NewText(fs, "text/kw/node/hb.unicast.intf.default"),
		Example:     "eth0",
		Option:      "intf",
		Scopable:    true,
		Section:     "hb",
		Text:        keywords.NewText(fs, "text/kw/node/hb.unicast.intf"),
		Types:       []string{"unicast"},
	}
	kwNodeHBUnicastPort = keywords.Keyword{
		Converter: "int",
		Default:   "10000",
		Option:    "port",
		Scopable:  true,
		Section:   "hb",
		Text:      keywords.NewText(fs, "text/kw/node/hb.unicast.port"),
		Types:     []string{"unicast"},
	}
	kwNodeHBTimeout = keywords.Keyword{
		Converter: "duration",
		Default:   "15s",
		Option:    "timeout",
		Scopable:  true,
		Section:   "hb",
		Text:      keywords.NewText(fs, "text/kw/node/hb.timeout"),
	}
	kwNodeHBInterval = keywords.Keyword{
		Converter: "duration",
		Default:   "5s",
		Option:    "interval",
		Scopable:  true,
		Section:   "hb",
		Text:      keywords.NewText(fs, "text/kw/node/hb.interval"),
	}
	kwNodeHBMulticastAddr = keywords.Keyword{
		Default:  "224.3.29.71",
		Option:   "addr",
		Scopable: true,
		Section:  "hb",
		Text:     keywords.NewText(fs, "text/kw/node/hb.multicast.addr"),
		Types:    []string{"multicast"},
	}
	kwNodeHBMulticastIntf = keywords.Keyword{
		DefaultText: keywords.NewText(fs, "text/kw/node/hb.multicast.intf.default"),
		Example:     "eth0",
		Option:      "intf",
		Scopable:    true,
		Section:     "hb",
		Text:        keywords.NewText(fs, "text/kw/node/hb.multicast.intf"),
		Types:       []string{"multicast"},
	}
	kwNodeHBMulticastPort = keywords.Keyword{
		Converter: "int",
		Default:   "10000",
		Option:    "port",
		Scopable:  true,
		Section:   "hb",
		Text:      keywords.NewText(fs, "text/kw/node/hb.multicast.port"),
		Types:     []string{"multicast"},
	}
	kwNodeHBUnicastNodes = keywords.Keyword{
		Converter:   "list",
		DefaultText: keywords.NewText(fs, "text/kw/node/hb.unicast.nodes.default"),
		Option:      "nodes",
		Scopable:    true,
		Section:     "hb",
		Text:        keywords.NewText(fs, "text/kw/node/hb.unicast.nodes"),
		Types:       []string{"unicast"},
	}
	kwNodeHBDiskDev = keywords.Keyword{
		Example:  "/dev/mapper/36589cfc000000e03957c51dabab8373a",
		Option:   "dev",
		Required: true,
		Scopable: true,
		Section:  "hb",
		Text:     keywords.NewText(fs, "text/kw/node/hb.disk.dev"),
		Types:    []string{"disk"},
	}
	kwNodeHBDiskMaxSlots = keywords.Keyword{
		Converter: "int",
		Example:   "1024",
		Default:   "1024",
		Option:    "max_slots",
		Section:   "hb",
		Text:      keywords.NewText(fs, "text/kw/node/hb.disk.max_slots"),
		Types:     []string{"disk"},
	}
	kwNodeHBRelayInsecure = keywords.Keyword{
		Converter: "bool",
		Default:   "false",
		Option:    "insecure",
		Section:   "hb",
		Text:      keywords.NewText(fs, "text/kw/node/hb.relay.insecure"),
		Types:     []string{"relay"},
	}
	kwNodeHBRelayRelay = keywords.Keyword{
		Example:  "https://relay.acme.com:1215",
		Option:   "relay",
		Required: true,
		Section:  "hb",
		Text:     keywords.NewText(fs, "text/kw/node/hb.relay.relay"),
		Types:    []string{"relay"},
	}
	kwNodeHBRelayUsername = keywords.Keyword{
		Default: "relay",
		Option:  "username",
		Section: "hb",
		Text:    keywords.NewText(fs, "text/kw/node/hb.relay.username"),
		Types:   []string{"relay"},
	}
	kwNodeHBRelayPassword = keywords.Keyword{
		Default: naming.NsSys + "/sec/relay",
		Option:  "password",
		Section: "hb",
		Example: "from system/sec/relays key relay.acme.com/user1/password",
		Text:    keywords.NewText(fs, "text/kw/node/hb.relay.password"),
		Types:   []string{"relay"},
	}
	kwNodeCNIPlugins = keywords.Keyword{
		Default: "/usr/lib/cni",
		Example: "/var/lib/opensvc/cni/bin",
		Option:  "plugins",
		Section: "cni",
		Text:    keywords.NewText(fs, "text/kw/node/cni.plugins"),
	}
	kwNodeCNIConfig = keywords.Keyword{
		Default: "/var/lib/opensvc/cni/net.d",
		Example: "/var/lib/opensvc/cni/net.d",
		Option:  "config",
		Section: "cni",
		Text:    keywords.NewText(fs, "text/kw/node/cni.config"),
	}
	kwNodePoolType = keywords.Keyword{
		Candidates: []string{"directory", "loop", "vg", "zpool", "freenas", "share", "shm", "symmetrix", "truenas", "virtual", "dorado", "hoc", "drbd", "pure", "rados"},
		Default:    "directory",
		Option:     "type",
		Section:    "pool",
		Text:       keywords.NewText(fs, "text/kw/node/pool.type"),
	}
	kwNodePoolSchedule = keywords.Keyword{
		Option:  kwoption.ScheduleStatus,
		Section: "pool",
		Text:    keywords.NewText(fs, "text/kw/node/pool.status_schedule"),
	}
	kwNodePoolMntOpt = keywords.Keyword{
		Option:   "mnt_opt",
		Scopable: true,
		Section:  "pool",
		Text:     keywords.NewText(fs, "text/kw/node/pool.mnt_opt"),
	}
	kwNodePoolArray = keywords.Keyword{
		Option:   "array",
		Required: true,
		Scopable: true,
		Section:  "pool",
		Text:     keywords.NewText(fs, "text/kw/node/pool.array"),
		Types:    []string{"freenas", "symmetrix", "dorado", "hoc", "pure", "truenas"},
	}
	kwNodePoolRadosRBDPool = keywords.Keyword{
		Option:   "rbd_pool",
		Section:  "pool",
		Required: true,
		Text:     "The ceph pool where to create images.",
		Types:    []string{"rados"},
	}
	kwNodePoolRadosRBDNamespace = keywords.Keyword{
		Option:  "rbd_namespace",
		Section: "pool",
		Text:    "The ceph pool namespace where to create images.",
		Types:   []string{"rados"},
	}
	kwNodePoolLabelPrefix = keywords.Keyword{
		Option:  "label_prefix",
		Section: "pool",
		Text:    keywords.NewText(fs, "text/kw/node/pool.hoc.label_prefix"),
		Types:   []string{"hoc", "pure"},
	}
	kwNodePoolPureDeleteNow = keywords.Keyword{
		Converter: "bool",
		Default:   "true",
		Option:    "delete_now",
		Section:   "pool",
		Text:      keywords.NewText(fs, "text/kw/node/delete_now.pure.pod"),
		Types:     []string{"pure"},
	}
	kwNodePoolPurePod = keywords.Keyword{
		Option:  "pod",
		Section: "pool",
		Text:    keywords.NewText(fs, "text/kw/node/pool.pure.pod"),
		Types:   []string{"pure"},
	}
	kwNodePoolPureVolumeGroup = keywords.Keyword{
		Option:  "volumegroup",
		Section: "pool",
		Text:    keywords.NewText(fs, "text/kw/node/pool.pure.volumegroup"),
		Types:   []string{"pure"},
	}
	kwNodePoolHOCWWIDPrefix = keywords.Keyword{
		Option:  "wwid_prefix",
		Section: "array",
		Types:   []string{"hoc"},
		Text:    keywords.NewText(fs, "text/kw/node/array.hoc.wwid_prefix"),
	}
	kwNodePoolHOCPool = keywords.Keyword{
		Option:  "volume_id_range_from",
		Section: "pool",
		Text:    keywords.NewText(fs, "text/kw/node/pool.hoc.volume_id_range_from"),
		Types:   []string{"hoc"},
	}
	kwNodePoolVolumeIDRangeTo = keywords.Keyword{
		Option:  "volume_id_range_to",
		Section: "pool",
		Text:    keywords.NewText(fs, "text/kw/node/pool.hoc.volume_id_range_to"),
		Types:   []string{"hoc"},
	}
	kwNodePoolHOCVSMID = keywords.Keyword{
		Default: "",
		Option:  "vsm_id",
		Section: "pool",
		Text:    keywords.NewText(fs, "text/kw/node/pool.hoc.vsm_id"),
		Types:   []string{"hoc"},
	}
	kwNodePoolSymmetrixSRP = keywords.Keyword{
		Option:   "srp",
		Required: true,
		Section:  "pool",
		Text:     keywords.NewText(fs, "text/kw/node/pool.symmetrix.srp"),
		Types:    []string{"symmetrix"},
	}
	kwNodePoolSymmetrixSLO = keywords.Keyword{
		Option:  "slo",
		Section: "pool",
		Text:    keywords.NewText(fs, "text/kw/node/pool.symmetrix.slo"),
		Types:   []string{"symmetrix"},
	}
	kwNodePoolSymmetrixSRDF = keywords.Keyword{
		Converter: "bool",
		Default:   "false",
		Option:    "srdf",
		Section:   "pool",
		Text:      keywords.NewText(fs, "text/kw/node/pool.symmetrix.srdf"),
		Types:     []string{"symmetrix"},
	}
	kwNodePoolSymmetrixRDFG = keywords.Keyword{
		Option:  "rdfg",
		Section: "pool",
		Text:    keywords.NewText(fs, "text/kw/node/pool.symmetrix.rdfg"),
		Types:   []string{"symmetrix"},
	}
	kwNodePoolDiskGroup = keywords.Keyword{
		Option:   "diskgroup",
		Required: true,
		Section:  "pool",
		Text:     keywords.NewText(fs, "text/kw/node/pool.diskgroup"),
		Types:    []string{"freenas", "dorado", "hoc", "pure", "truenas"},
	}
	kwNodePoolTruenasInsecureTPC = keywords.Keyword{
		Converter: "bool",
		Default:   "false",
		Option:    "insecure_tpc",
		Section:   "pool",
		Text:      keywords.NewText(fs, "text/kw/node/pool.freenas.insecure_tpc"),
		Types:     []string{"freenas", "truenas"},
	}
	kwNodePoolTruenasCompression = keywords.Keyword{
		Candidates: []string{"inherit", "none", "lz4", "gzip-1", "gzip-2", "gzip-3", "gzip-4", "gzip-5", "gzip-6", "gzip-7", "gzip-8", "gzip-9", "zle", "lzjb"},
		Default:    "inherit",
		Option:     "compression",
		Section:    "pool",
		Text:       keywords.NewText(fs, "text/kw/node/pool.freenas.compression"),
		Types:      []string{"freenas", "truenas"},
	}
	kwNodePoolTruenasSparse = keywords.Keyword{
		Default:   "false",
		Converter: "bool",
		Option:    "sparse",
		Section:   "pool",
		Text:      keywords.NewText(fs, "text/kw/node/pool.freenas.sparse"),
		Types:     []string{"freenas", "truenas"},
	}
	kwNodePoolTruenasBlockSize = keywords.Keyword{
		Converter: "size",
		Default:   "512",
		Option:    "blocksize",
		Section:   "pool",
		Text:      keywords.NewText(fs, "text/kw/node/pool.freenas.blocksize"),
		Types:     []string{"freenas", "truenas"},
	}
	kwNodePoolName = keywords.Keyword{
		Option:   "name",
		Required: true,
		Section:  "pool",
		Text:     keywords.NewText(fs, "text/kw/node/pool.vg.name"),
		Types:    []string{"vg"},
	}
	kwNodePoolDRBDAddr = keywords.Keyword{
		DefaultText: keywords.NewText(fs, "text/kw/node/pool.drbd.addr.default"),
		Example:     "1.2.3.4",
		Option:      "addr",
		Section:     "pool",
		Scopable:    true,
		Text:        keywords.NewText(fs, "text/kw/node/pool.drbd.addr"),
		Types:       []string{"drbd"},
	}
	kwNodePoolDRBDTemplate = keywords.Keyword{
		Attr:    "Template",
		Example: "live-migration",
		Option:  "template",
		Section: "pool",
		Text:    "The value of the template keyword to set in the drbd resource of the created volumes",
		Types:   []string{"drbd"},
	}
	kwNodePoolDRBDVG = keywords.Keyword{
		Option:  "vg",
		Section: "pool",
		Text:    keywords.NewText(fs, "text/kw/node/pool.drbd.vg"),
		Types:   []string{"drbd"},
	}
	kwNodePoolZpoolName = keywords.Keyword{
		Option:   "name",
		Required: true,
		Section:  "pool",
		Text:     keywords.NewText(fs, "text/kw/node/pool.zpool.name"),
		Types:    []string{"zpool"},
	}
	kwNodePoolZpoolPool = keywords.Keyword{
		Option:  "zpool",
		Section: "pool",
		Text:    keywords.NewText(fs, "text/kw/node/pool.drbd.zpool"),
		Types:   []string{"drbd"},
	}
	kwNodePoolZpoolPath = keywords.Keyword{
		Option:  "path",
		Section: "pool",
		Text:    keywords.NewText(fs, "text/kw/node/pool.drbd.path"),
		Types:   []string{"drbd"},
	}
	kwNodePoolSharePath = keywords.Keyword{
		Default: "{var}/pool/share",
		Option:  "path",
		Section: "pool",
		Text:    keywords.NewText(fs, "text/kw/node/pool.share.path"),
		Types:   []string{"share"},
	}
	kwNodePoolDirectoryPath = keywords.Keyword{
		Default: "{var}/pool/directory",
		Option:  "path",
		Section: "pool",
		Text:    keywords.NewText(fs, "text/kw/node/pool.directory.path"),
		Types:   []string{"directory"},
	}
	kwNodePoolVirtualTemplate = keywords.Keyword{
		Example: "templates/vol/mpool-over-loop",
		Option:  "template",
		Section: "pool",
		Text:    keywords.NewText(fs, "text/kw/node/pool.virtual.template"),
		Types:   []string{"virtual"},
	}
	kwNodePoolVirtualVolumeEnv = keywords.Keyword{
		Converter: "list",
		Example:   "container#1.name:container_name env.foo:foo",
		Option:    "volume_env",
		Section:   "pool",
		Text:      keywords.NewText(fs, "text/kw/node/pool.virtual.volume_env"),
		Types:     []string{"virtual"},
	}
	kwNodePoolVirtualOptionalVolumeEnv = keywords.Keyword{
		Converter: "list",
		Example:   "container#1.name:container_name env.foo:foo",
		Option:    "optional_volume_env",
		Section:   "pool",
		Text:      keywords.NewText(fs, "text/kw/node/pool.virtual.optional_volume_env"),
		Types:     []string{"virtual"},
	}
	kwNodePoolVirtualCapabilities = keywords.Keyword{
		Converter: "list",
		Default:   "file roo rwo rox rwx",
		Option:    "capabilities",
		Section:   "pool",
		Text:      keywords.NewText(fs, "text/kw/node/pool.virtual.capabilities"),
		Types:     []string{"virtual"},
	}
	kwNodePoolLoopPath = keywords.Keyword{
		Default: "{var}/pool/loop",
		Option:  "path",
		Section: "pool",
		Text:    keywords.NewText(fs, "text/kw/node/pool.loop.path"),
		Types:   []string{"loop"},
	}
	kwNodePoolHOCPoolID = keywords.Keyword{
		Default: "",
		Option:  "pool_id",
		Section: "pool",
		Text:    keywords.NewText(fs, "text/kw/node/pool.hoc.pool_id"),
		Types:   []string{"hoc"},
	}
	kwNodePoolFSType = keywords.Keyword{
		Default: "xfs",
		Option:  "fs_type",
		Section: "pool",
		Text:    keywords.NewText(fs, "text/kw/node/pool.fs_type"),
		Types:   []string{"freenas", "dorado", "hoc", "symmetrix", "drbd", "loop", "vg", "pure", "truenas", "rados"},
	}
	kwNodePoolMkfsOpt = keywords.Keyword{
		Example: "-O largefile",
		Option:  "mkfs_opt",
		Section: "pool",
		Text:    keywords.NewText(fs, "text/kw/node/pool.mkfs_opt"),
	}
	kwNodePoolMkblkOpt = keywords.Keyword{
		Option:  "mkblk_opt",
		Section: "pool",
		Text:    keywords.NewText(fs, "text/kw/node/pool.mkblk_opt"),
	}
	kwNodeHookEvents = keywords.Keyword{
		Converter: "list",
		Option:    "events",
		Section:   "hook",
		Text:      keywords.NewText(fs, "text/kw/node/hook.events"),
	}
	kwNodeHookCommand = keywords.Keyword{
		Converter: "shlex",
		Option:    "command",
		Section:   "hook",
		Text:      keywords.NewText(fs, "text/kw/node/hook.command"),
	}
	kwNodeNetworkType = keywords.Keyword{
		Candidates: []string{"bridge", "routed_bridge"},
		Default:    "bridge",
		Option:     "type",
		Section:    "network",
		Text:       keywords.NewText(fs, "text/kw/node/network.type"),
	}
	kwNodeNetworkRoutedBridgeSubnet = keywords.Keyword{
		Option:   "subnet",
		Section:  "network",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/node/network.routed_bridge.subnet"),
		Types:    []string{"routed_bridge"},
	}
	kwNodeNetworkRoutedBridgeGateway = keywords.Keyword{
		Option:   "gateway",
		Scopable: true,
		Section:  "network",
		Text:     keywords.NewText(fs, "text/kw/node/network.routed_bridge.gateway"),
		Types:    []string{"routed_bridge"},
	}
	kwNodeNetworkRoutedBridgeIPsPerNode = keywords.Keyword{
		Converter:  "int",
		Default:    "1024",
		Deprecated: "3.0.0",
		Option:     "ips_per_node",
		Section:    "network",
		Text:       keywords.NewText(fs, "text/kw/node/network.routed_bridge.ips_per_node"),
		Types:      []string{"routed_bridge"},
	}
	kwNodeNetworkRoutedBridgeMaskPerNode = keywords.Keyword{
		Converter: "int",
		Default:   "0",
		Option:    "mask_per_node",
		Section:   "network",
		Text:      keywords.NewText(fs, "text/kw/node/network.routed_bridge.mask_per_node"),
	}
	kwNodeNetworkRoutedBridgeTables = keywords.Keyword{
		Converter: "list",
		Default:   "main",
		Example:   "main custom1 custom2",
		Option:    "tables",
		Section:   "network",
		Text:      keywords.NewText(fs, "text/kw/node/network.routed_bridge.tables"),
		Types:     []string{"routed_bridge"},
	}
	kwNodeNetworkRoutedBridgeAddr = keywords.Keyword{
		DefaultText: keywords.NewText(fs, "text/kw/node/network.routed_bridge.addr.default"),
		Option:      "addr",
		Section:     "network",
		Scopable:    true,
		Text:        keywords.NewText(fs, "text/kw/node/network.routed_bridge.addr"),
		Types:       []string{"routed_bridge"},
	}
	kwNodeNetworkRoutedBridgeTunnel = keywords.Keyword{
		Candidates: []string{"auto", "always", "never"},
		Default:    "auto",
		Option:     "tunnel",
		Section:    "network",
		Text:       keywords.NewText(fs, "text/kw/node/network.routed_bridge.tunnel"),
		Types:      []string{"routed_bridge"},
	}
	kwNodeNetworkRoutedBridgeTunnelMode = keywords.Keyword{
		Candidates: []string{"gre", "ipip", "ip6ip6"},
		Default:    "ipip",
		Option:     "tunnel_mode",
		Section:    "network",
		Text:       keywords.NewText(fs, "text/kw/node/network.routed_bridge.tunnel_mode"),
		Types:      []string{"routed_bridge"},
	}
	kwNodeNetworkBridgeNetwork = keywords.Keyword{
		Option:   "network",
		Section:  "network",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/node/network.network"),
		Types:    []string{"bridge"},
	}
	kwNodeNetworkRoutedBridgeNetwork = keywords.Keyword{
		Option:  "network",
		Section: "network",
		Text:    keywords.NewText(fs, "text/kw/node/network.network"),
		Types:   []string{"routed_bridge"},
	}
	kwNodeNetworkDev = keywords.Keyword{
		Option:  "dev",
		Section: "network",
		Text:    keywords.NewText(fs, "text/kw/node/network.dev"),
		Types:   []string{"bridge", "routed_bridge"},
	}
	kwNodeNetworkPublic = keywords.Keyword{
		Converter: "bool",
		Option:    "public",
		Section:   "network",
		Text:      keywords.NewText(fs, "text/kw/node/network.public"),
		Types:     []string{"bridge", "routed_bridge"},
	}
	kwNodeSwitchType = keywords.Keyword{
		Candidates: []string{"brocade"},
		Option:     "type",
		Required:   true,
		Section:    "switch",
		Text:       keywords.NewText(fs, "text/kw/node/switch.type"),
	}
	kwNodeSwitchName = keywords.Keyword{
		Example: "sansw1.my.corp",
		Option:  "name",
		Section: "switch",
		Text:    keywords.NewText(fs, "text/kw/node/switch.brocade.name"),
		Types:   []string{"brocade"},
	}
	kwNodeSwitchMethod = keywords.Keyword{
		Candidates: []string{"telnet", "ssh"},
		Default:    "ssh",
		Example:    "ssh",
		Option:     "method",
		Section:    "switch",
		Text:       keywords.NewText(fs, "text/kw/node/switch.brocade.method"),
		Types:      []string{"brocade"},
	}
	kwNodeSwitchUsername = keywords.Keyword{
		Example:  "admin",
		Option:   "username",
		Required: true,
		Section:  "switch",
		Text:     keywords.NewText(fs, "text/kw/node/switch.brocade.username"),
		Types:    []string{"brocade"},
	}
	kwNodeSwitchPassword = keywords.Keyword{
		Example: "mysec/password",
		Option:  "password",
		Section: "switch",
		Text:    keywords.NewText(fs, "text/kw/node/switch.brocade.password"),
		Types:   []string{"brocade"},
	}
	kwNodeSwitchKey = keywords.Keyword{
		Example: "/path/to/key",
		Option:  "key",
		Section: "switch",
		Text:    keywords.NewText(fs, "text/kw/node/switch.brocade.key"),
		Types:   []string{"brocade"},
	}
	kwNodeArrayType = keywords.Keyword{
		Candidates: []string{"freenas", "hds", "eva", "nexenta", "vioserver", "centera", "symmetrix", "emcvnx", "netapp", "hp3par", "ibmds", "ibmsvc", "xtremio", "dorado", "hoc", "truenas"},
		Option:     "type",
		Required:   true,
		Section:    "array",
		Text:       keywords.NewText(fs, "text/kw/node/array.type"),
	}
	kwNodePoolCompression = keywords.Keyword{
		Converter: "bool",
		Default:   "false",
		Option:    "compression",
		Section:   "pool",
		Text:      keywords.NewText(fs, "text/kw/node/pool.compression"),
		Types:     []string{"dorado", "hoc"},
	}
	kwNodePoolTruenasDedup = keywords.Keyword{
		Default: "off",
		Option:  "dedup",
		Section: "pool",
		Text:    keywords.NewText(fs, "text/kw/node/pool.freenas.dedup"),
		Types:   []string{"freenas", "truenas"},
	}
	kwNodePoolDedup = keywords.Keyword{
		Converter: "bool",
		Default:   "false",
		Option:    "dedup",
		Section:   "pool",
		Text:      keywords.NewText(fs, "text/kw/node/pool.dedup"),
		Types:     []string{"dorado", "hoc"},
	}
	kwNodePoolDoradoHypermetroDomain = keywords.Keyword{
		Option:   "hypermetrodomain",
		Example:  "HyperMetroDomain_000",
		Required: false,
		Section:  "pool",
		Text:     keywords.NewText(fs, "text/kw/node/pool.dorado.hypermetrodomain"),
		Types:    []string{"dorado"},
	}
	kwNodePoolAPI = keywords.Keyword{
		Example:  "https://array.opensvc.com/api/v1.0",
		Option:   "api",
		Required: true,
		Section:  "array",
		Text:     keywords.NewText(fs, "text/kw/node/array.api"),
		Types:    []string{"dorado", "freenas", "hoc", "pure", "truenas", "xtremio"},
	}
	kwNodePoolHOCHTTPProxy = keywords.Keyword{
		Example: "http://proxy.mycorp:3158",
		Option:  "http_proxy",
		Section: "array",
		Text:    keywords.NewText(fs, "text/kw/node/array.hoc.http_proxy"),
		Types:   []string{"hoc"},
	}
	kwNodePoolHOCHTTPSProxy = keywords.Keyword{
		Example: "https://proxy.mycorp:3158",
		Option:  "https_proxy",
		Section: "array",
		Text:    keywords.NewText(fs, "text/kw/node/array.hoc.https_proxy"),
		Types:   []string{"hoc"},
	}
	kwNodeArrayHOCRetry = keywords.Keyword{
		Converter: "int",
		Default:   "30",
		Option:    "retry",
		Section:   "array",
		Text:      keywords.NewText(fs, "text/kw/node/array.hoc.retry"),
		Types:     []string{"hoc"},
	}
	kwNodeArrayHOCDelay = keywords.Keyword{
		Converter: "duration",
		Default:   "10s",
		Option:    "delay",
		Section:   "array",
		Text:      keywords.NewText(fs, "text/kw/node/array.hoc.delay"),
		Types:     []string{"hoc"},
	}
	kwNodeArrayHOCModel = keywords.Keyword{
		Candidates: []string{"VSP G370", "VSP G700", "VSP G900", "VSP F370", "VSP F700", "VSP F900", "VSP G350", "VSP F350", "VSP G800", "VSP F800", "VSP G400", "VSP G600", "VSP F400", "VSP F600", "VSP G200", "VSP G1000", "VSP G1500", "VSP F1500", "Virtual Storage Platform", "HUS VM"},
		Example:    "VSP G350",
		Option:     "model",
		Required:   true,
		Section:    "array",
		Text:       keywords.NewText(fs, "text/kw/node/array.hoc.model"),
		Types:      []string{"hoc"},
	}
	kwNodeArrayUsernameRequired = keywords.Keyword{
		Example:  "root",
		Option:   "username",
		Required: true,
		Section:  "array",
		Text:     keywords.NewText(fs, "text/kw/node/array.username,required"),
		Types:    []string{"centera", "eva", "hds", "ibmds", "ibmsvc", "freenas", "netapp", "nexenta", "vioserver", "xtremio", "dorado", "hoc", "truenas"},
	}
	kwNodeArrayUsername = keywords.Keyword{
		Example: "root",
		Option:  "username",
		Section: "array",
		Text:    keywords.NewText(fs, "text/kw/node/array.username,optional"),
		Types:   []string{"emcvnx", "hp3par", "symmetrix"},
	}
	kwNodeArrayPureClientID = keywords.Keyword{
		Example:  "bd2c75d0-f0d5-11ee-a362-8b0f2d1b83d7",
		Option:   "client_id",
		Required: true,
		Section:  "array",
		Text:     keywords.NewText(fs, "text/kw/node/array.pure.client_id"),
		Types:    []string{"pure"},
	}
	kwNodeArrayPureKeyID = keywords.Keyword{
		Example:  "df80ae3a-f0d5-11ee-94c9-b7c8d2f57c4f",
		Option:   "key_id",
		Required: true,
		Section:  "array",
		Text:     keywords.NewText(fs, "text/kw/node/array.pure.key_id"),
		Types:    []string{"pure"},
	}
	kwNodeArrayInsecure = keywords.Keyword{
		Converter: "bool",
		Default:   "false",
		Option:    "insecure",
		Example:   "true",
		Section:   "array",
		Text:      keywords.NewText(fs, "text/kw/node/array.pure.insecure"),
		Types:     []string{"pure", "hoc", "freenas", "truenas"},
	}
	kwNodeArrayPureIssuer = keywords.Keyword{
		Example:  "opensvc",
		Option:   "issuer",
		Required: true,
		Section:  "array",
		Text:     keywords.NewText(fs, "text/kw/node/array.pure.issuer"),
		Types:    []string{"pure"},
	}
	kwNodeArrayPureSecret = keywords.Keyword{
		Example:  naming.NsSys + "/sec/array1",
		Option:   "secret",
		Required: true,
		Section:  "array",
		Text:     keywords.NewText(fs, "text/kw/node/array.pure.secret"),
		Types:    []string{"pure"},
	}
	kwNodeArrayPureUsername = keywords.Keyword{
		Example:  "opensvc",
		Option:   "username",
		Required: true,
		Section:  "array",
		Text:     keywords.NewText(fs, "text/kw/node/array.pure.username"),
		Types:    []string{"pure"},
	}
	kwNodeArrayPasswordRequired = keywords.Keyword{
		Example:  "from system/sec/array1 key password",
		Option:   "password",
		Required: true,
		Section:  "array",
		Text:     keywords.NewText(fs, "text/kw/node/array.password,required"),
		Types:    []string{"centera", "eva", "hds", "freenas", "nexenta", "xtremio", "truenas", "dorado", "hoc"},
	}
	kwNodeArrayPasswordOptional = keywords.Keyword{
		Example: naming.NsSys + "/sec/array1",
		Option:  "password",
		Section: "array",
		Text:    keywords.NewText(fs, "text/kw/node/array.password,optional"),
		Types:   []string{"emcvnx", "symmetrix"},
	}
	kwNodeArrayTimeout = keywords.Keyword{
		Converter: "duration",
		Default:   "120s",
		Example:   "10s",
		Option:    "timeout",
		Section:   "array",
		Text:      keywords.NewText(fs, "text/kw/node/array.timeout"),
		Types:     []string{"freenas", "dorado", "hoc", "truenas"},
	}
	kwNodeArrayName = keywords.Keyword{
		Example: "a09",
		Option:  "name",
		Section: "array",
		Text:    keywords.NewText(fs, "text/kw/node/array.name"),
		Types:   []string{"dorado", "hoc"},
	}
	kwNodeArraySymmetrixName = keywords.Keyword{
		Example: "00012345",
		Option:  "name",
		Section: "array",
		Text:    keywords.NewText(fs, "text/kw/node/array.symmetrix.name"),
		Types:   []string{"symmetrix"},
	}
	kwNodeArraySymmetrixSymcliPath = keywords.Keyword{
		Default: "/usr/symcli",
		Example: "/opt/symcli",
		Option:  "symcli_path",
		Section: "array",
		Text:    keywords.NewText(fs, "text/kw/node/array.symmetrix.symcli_path"),
		Types:   []string{"symmetrix"},
	}
	kwNodeArraySymmetrixSymcliConnect = keywords.Keyword{
		Example: "MY_SYMAPI_SERVER",
		Option:  "symcli_connect",
		Section: "array",
		Text:    keywords.NewText(fs, "text/kw/node/array.symmetrix.symcli_connect"),
		Types:   []string{"symmetrix"},
	}
	kwNodeArrayServer = keywords.Keyword{
		Example:  "centera1",
		Option:   "server",
		Required: true,
		Section:  "array",
		Text:     keywords.NewText(fs, "text/kw/node/array.server"),
		Types:    []string{"centera", "netapp"},
	}
	kwNodeArrayCenteraJavaBin = keywords.Keyword{
		Example:  "/opt/java/bin/java",
		Option:   "java_bin",
		Required: true,
		Section:  "array",
		Text:     keywords.NewText(fs, "text/kw/node/array.centera.java_bin"),
		Types:    []string{"centera"},
	}
	kwNodeArrayCenteraJcassDir = keywords.Keyword{
		Example:  "/opt/centera/LIB",
		Option:   "jcass_dir",
		Required: true,
		Section:  "array",
		Text:     keywords.NewText(fs, "text/kw/node/array.centera.jcass_dir"),
		Types:    []string{"centera"},
	}
	kwNodeArrayEMCVNXSecFile = keywords.Keyword{
		Example:    "secfile",
		Candidates: []string{"secfile", "credentials"},
		Default:    "secfile",
		Option:     "method",
		Section:    "array",
		Text:       keywords.NewText(fs, "text/kw/node/array.emcvnx.secfile"),
		Types:      []string{"emcvnx"},
	}
	kwNodeArrayEMCVNXSPA = keywords.Keyword{
		Example:  "array1-a",
		Option:   "spa",
		Required: true,
		Section:  "array",
		Text:     keywords.NewText(fs, "text/kw/node/array.emcvnx.spa"),
		Types:    []string{"emcvnx"},
	}
	kwNodeArrayEMCVNXSPB = keywords.Keyword{
		Example:  "array1-b",
		Option:   "spb",
		Required: true,
		Section:  "array",
		Text:     keywords.NewText(fs, "text/kw/node/array.emcvnx.spb"),
		Types:    []string{"emcvnx"},
	}
	kwNodeArrayEMCVNXScope = keywords.Keyword{
		Default: "0",
		Example: "1",
		Option:  "scope",
		Section: "array",
		Text:    keywords.NewText(fs, "text/kw/node/array.emcvnx.scope"),
		Types:   []string{"emcvnx"},
	}
	kwNodeArrayEVAManager = keywords.Keyword{
		Example:  "evamanager.mycorp",
		Option:   "manager",
		Required: true,
		Section:  "array",
		Text:     keywords.NewText(fs, "text/kw/node/array.eva.manager"),
		Types:    []string{"eva"},
	}
	kwNodeArrayEVABin = keywords.Keyword{
		Example: "/opt/sssu/bin/sssu",
		Option:  "bin",
		Section: "array",
		Text:    keywords.NewText(fs, "text/kw/node/array.eva.bin"),
		Types:   []string{"eva"},
	}
	kwNodeArrayHDSBin = keywords.Keyword{
		Example: "/opt/hds/bin/HiCommandCLI",
		Option:  "bin",
		Section: "array",
		Text:    keywords.NewText(fs, "text/kw/node/array.hds.bin"),
		Types:   []string{"hds"},
	}
	kwNodeArrayHDSJREPath = keywords.Keyword{
		Example: "/opt/java",
		Option:  "jre_path",
		Section: "array",
		Text:    keywords.NewText(fs, "text/kw/node/array.hds.jre_path"),
		Types:   []string{"hds"},
	}
	kwNodeArrayHDSName = keywords.Keyword{
		Option:  "name",
		Example: "HUSVM.1234",
		Section: "array",
		Text:    keywords.NewText(fs, "text/kw/node/array.hds.name"),
		Types:   []string{"hds"},
	}
	kwNodeArrayHDSURL = keywords.Keyword{
		Example:  "https://hdsmanager/",
		Option:   "url",
		Required: true,
		Section:  "array",
		Text:     keywords.NewText(fs, "text/kw/node/array.hds.url"),
		Types:    []string{"hds"},
	}
	kwNodeArrayHP3PARMethod = keywords.Keyword{
		Candidates: []string{"proxy", "cli", "ssh"},
		Default:    "ssh",
		Example:    "ssh",
		Option:     "method",
		Section:    "array",
		Text:       keywords.NewText(fs, "text/kw/node/array.hp3par.method"),
		Types:      []string{"hp3par"},
	}
	kwNodeArrayHP3PARManager = keywords.Keyword{
		DefaultText: keywords.NewText(fs, "text/kw/node/array.hp3par.manager.default"),
		Example:     "mymanager.mycorp",
		Option:      "manager",
		Section:     "array",
		Text:        keywords.NewText(fs, "text/kw/node/array.hp3par.manager"),
		Types:       []string{"hp3par"},
	}
	kwNodeArrayHP3PARKey = keywords.Keyword{
		Example: "/path/to/key",
		Option:  "key",
		Section: "array",
		Text:    keywords.NewText(fs, "text/kw/node/array.hp3par.key"),
		Types:   []string{"hp3par"},
	}
	kwNodeArrayHP3PARPwf = keywords.Keyword{
		Example: "/path/to/pwf",
		Option:  "pwf",
		Section: "array",
		Text:    keywords.NewText(fs, "text/kw/node/array.hp3par.pwf"),
		Types:   []string{"hp3par"},
	}
	kwNodeArrayHP3PARCLI = keywords.Keyword{
		Default: "3parcli",
		Example: "/path/to/pwf",
		Option:  "cli",
		Section: "array",
		Text:    keywords.NewText(fs, "text/kw/node/array.hp3par.cli"),
		Types:   []string{"hp3par"},
	}
	kwNodeArrayIBMDSHMC1 = keywords.Keyword{
		Example:  "hmc1.mycorp",
		Option:   "hmc1",
		Required: true,
		Section:  "array",
		Text:     keywords.NewText(fs, "text/kw/node/array.ibmds.hmc1"),
		Types:    []string{"ibmds"},
	}
	kwNodeArrayIBMDSHMC2 = keywords.Keyword{
		Example:  "hmc2.mycorp",
		Option:   "hmc2",
		Required: true,
		Section:  "array",
		Text:     keywords.NewText(fs, "text/kw/node/array.ibmds.hmc2"),
		Types:    []string{"ibmds"},
	}
	kwNodeArrayKeyRequired = keywords.Keyword{
		Example:  "/path/to/key",
		Option:   "key",
		Required: true,
		Section:  "array",
		Text:     keywords.NewText(fs, "text/kw/node/array.key,required"),
		Types:    []string{"netapp", "ibmsvc", "vioserver"},
	}
	kwNodeArrayNexentaPort = keywords.Keyword{
		Converter: "int",
		Default:   "2000",
		Example:   "2000",
		Option:    "port",
		Section:   "array",
		Text:      keywords.NewText(fs, "text/kw/node/array.nexenta.port"),
		Types:     []string{"nexenta"},
	}

	nodeCommonKeywords = []*keywords.Keyword{
		&kwNodeSecureFetch,
		&kwNodeMinAvailMemPct,
		&kwNodeMinAvailSwapPct,
		&kwNodeEnv,
		&kwNodeConsoleMaxSeats,
		&kwNodeConsoleInsecure,
		&kwNodeConsoleServer,
		&kwNodeMaxParallel,
		&kwNodeMaxKeySize,
		&kwNodeAllowedNetworks,
		&kwNodeLocCountry,
		&kwNodeLocCity,
		&kwNodeLocZIP,
		&kwNodeLocAddr,
		&kwNodeLocBuilding,
		&kwNodeLocFloor,
		&kwNodeLocRoom,
		&kwNodeLocRack,
		&kwNodeSecZone,
		&kwNodeTeamInteg,
		&kwNodeTeamSupport,
		&kwNodeAssetEnv,
		&kwNodeDBOpensvc,
		&kwNodeDBInsecure,
		&kwNodeDBCompliance,
		&kwNodeDBLog,
		&kwNodeCollector,
		&kwNodeCollectorServer,
		&kwNodeCollectorFeeder,
		&kwNodeCollectorTimeout,
		&kwNodeBranch,
		&kwNodeRepo,
		&kwNodeRepoPkg,
		&kwNodeRepoComp,
		&kwNodeRUser,
		&kwNodeMaintenanceGracePeriod,
		&kwNodeRejoinGracePeriod,
		&kwNodeReadyPeriod,
		&kwNodeDequeueActionSchedule,
		&kwNodeSysreportSchedule,
		&kwNodeComplianceSchedule,
		&kwNodeArraySchedule,
		&kwNodeArrayXtremioName,
		&kwNodeComplianceAutoUpdate,
		&kwNodeChecksSchedule,
		&kwNodePackagesSchedule,
		&kwNodePatchesSchedule,
		&kwNodeAssetSchedule,
		&kwNodeDisksSchedule,
		&kwNodeListenerCRL,
		&kwNodeListenerDNSSockUID,
		&kwNodeListenerDNSSockGID,
		&kwNodeListenerAddr,
		&kwNodeListenerPort,
		&kwNodeListenerOpenIDIssuer,
		&kwNodeListenerOpenIDClientID,
		&kwNodeListenerRateLimiterRate,
		&kwNodeListenerRateLimiterBurst,
		&kwNodeListenerRateLimiterExpires,
		&kwNodeSyslogFacility,
		&kwNodeSyslogLevel,
		&kwNodeSyslogHost,
		&kwNodeSyslogPort,
		&kwNodeClusterDNS,
		&kwNodeClusterCA,
		&kwNodeClusterCert,
		&kwNodeClusterID,
		&kwNodeClusterName,
		&kwNodeClusterSecret,
		&kwNodeClusterNodes,
		&kwNodeClusterDRPNodes,
		&kwNodeClusterEnvs,
		&kwNodeClusterQuorum,
		&kwNodeSSHKey,
		&kwNodeSplitAction,
		&kwNodeArbitratorURI,
		&kwNodeArbitratorInsecure,
		&kwNodeArbitratorWeight,
		&kwNodeStonithCommand,
		&kwNodeHBType,
		&kwNodeHBUnicastAddr,
		&kwNodeHBUnicastIntf,
		&kwNodeHBUnicastPort,
		&kwNodeHBTimeout,
		&kwNodeHBInterval,
		&kwNodeHBMulticastAddr,
		&kwNodeHBMulticastIntf,
		&kwNodeHBMulticastPort,
		&kwNodeHBUnicastNodes,
		&kwNodeHBDiskDev,
		&kwNodeHBDiskMaxSlots,
		&kwNodeHBRelayInsecure,
		&kwNodeHBRelayRelay,
		&kwNodeHBRelayUsername,
		&kwNodeHBRelayPassword,
		&kwNodeCNIPlugins,
		&kwNodeCNIConfig,
		&kwNodePoolType,
		&kwNodePoolSchedule,
		&kwNodePoolMntOpt,
		&kwNodePoolArray,
		&kwNodePoolRadosRBDPool,
		&kwNodePoolRadosRBDNamespace,
		&kwNodePoolLabelPrefix,
		&kwNodePoolPureDeleteNow,
		&kwNodePoolPurePod,
		&kwNodePoolPureVolumeGroup,
		&kwNodePoolHOCWWIDPrefix,
		&kwNodePoolHOCPool,
		&kwNodePoolVolumeIDRangeTo,
		&kwNodePoolHOCVSMID,
		&kwNodePoolSymmetrixSRP,
		&kwNodePoolSymmetrixSLO,
		&kwNodePoolSymmetrixSRDF,
		&kwNodePoolSymmetrixRDFG,
		&kwNodePoolDiskGroup,
		&kwNodePoolTruenasInsecureTPC,
		&kwNodePoolTruenasCompression,
		&kwNodePoolTruenasSparse,
		&kwNodePoolTruenasBlockSize,
		&kwNodePoolName,
		&kwNodePoolDRBDAddr,
		&kwNodePoolDRBDTemplate,
		&kwNodePoolDRBDVG,
		&kwNodePoolZpoolName,
		&kwNodePoolZpoolPool,
		&kwNodePoolZpoolPath,
		&kwNodePoolSharePath,
		&kwNodePoolDirectoryPath,
		&kwNodePoolVirtualTemplate,
		&kwNodePoolVirtualVolumeEnv,
		&kwNodePoolVirtualOptionalVolumeEnv,
		&kwNodePoolVirtualCapabilities,
		&kwNodePoolLoopPath,
		&kwNodePoolHOCPoolID,
		&kwNodePoolFSType,
		&kwNodePoolMkfsOpt,
		&kwNodePoolMkblkOpt,
		&kwNodeHookEvents,
		&kwNodeHookCommand,
		&kwNodeNetworkType,
		&kwNodeNetworkRoutedBridgeSubnet,
		&kwNodeNetworkRoutedBridgeGateway,
		&kwNodeNetworkRoutedBridgeIPsPerNode,
		&kwNodeNetworkRoutedBridgeMaskPerNode,
		&kwNodeNetworkRoutedBridgeTables,
		&kwNodeNetworkRoutedBridgeAddr,
		&kwNodeNetworkRoutedBridgeTunnel,
		&kwNodeNetworkRoutedBridgeTunnelMode,
		&kwNodeNetworkBridgeNetwork,
		&kwNodeNetworkRoutedBridgeNetwork,
		&kwNodeNetworkDev,
		&kwNodeNetworkPublic,
		&kwNodeSwitchType,
		&kwNodeSwitchName,
		&kwNodeSwitchMethod,
		&kwNodeSwitchUsername,
		&kwNodeSwitchPassword,
		&kwNodeSwitchKey,
		&kwNodeArrayType,
		&kwNodePoolCompression,
		&kwNodePoolTruenasDedup,
		&kwNodePoolDedup,
		&kwNodePoolDoradoHypermetroDomain,
		&kwNodePoolAPI,
		&kwNodePoolHOCHTTPProxy,
		&kwNodePoolHOCHTTPSProxy,
		&kwNodeArrayHOCRetry,
		&kwNodeArrayHOCDelay,
		&kwNodeArrayHOCModel,
		&kwNodeArrayUsernameRequired,
		&kwNodeArrayUsername,
		&kwNodeArrayPureClientID,
		&kwNodeArrayPureKeyID,
		&kwNodeArrayInsecure,
		&kwNodeArrayPureIssuer,
		&kwNodeArrayPureSecret,
		&kwNodeArrayPureUsername,
		&kwNodeArrayPasswordRequired,
		&kwNodeArrayPasswordOptional,
		&kwNodeArrayTimeout,
		&kwNodeArrayName,
		&kwNodeArraySymmetrixName,
		&kwNodeArraySymmetrixSymcliPath,
		&kwNodeArraySymmetrixSymcliConnect,
		&kwNodeArrayServer,
		&kwNodeArrayCenteraJavaBin,
		&kwNodeArrayCenteraJcassDir,
		&kwNodeArrayEMCVNXSecFile,
		&kwNodeArrayEMCVNXSPA,
		&kwNodeArrayEMCVNXSPB,
		&kwNodeArrayEMCVNXScope,
		&kwNodeArrayEVAManager,
		&kwNodeArrayEVABin,
		&kwNodeArrayHDSBin,
		&kwNodeArrayHDSJREPath,
		&kwNodeArrayHDSName,
		&kwNodeArrayHDSURL,
		&kwNodeArrayHP3PARMethod,
		&kwNodeArrayHP3PARManager,
		&kwNodeArrayHP3PARKey,
		&kwNodeArrayHP3PARPwf,
		&kwNodeArrayHP3PARCLI,
		&kwNodeArrayIBMDSHMC1,
		&kwNodeArrayIBMDSHMC2,
		&kwNodeArrayKeyRequired,
		&kwNodeArrayNexentaPort,
	}
)

var NodeKeywordStore = keywords.Store(append(nodePrivateKeywords, nodeCommonKeywords...))

func (t Node) KeywordLookup(k key.T, sectionType string) *keywords.Keyword {
	return keywordLookup(NodeKeywordStore, k, naming.KindInvalid, sectionType)
}
