package manifest

import (
	"embed"

	"github.com/opensvc/om3/v3/core/keywords"
)

//go:embed text
var fs embed.FS

var (
	KWBlockingPostProvision = keywords.Keyword{
		Attr:     "BlockingPostProvision",
		Option:   "blocking_post_provision",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/blocking_post_provision"),
	}

	KWBlockingPostRun = keywords.Keyword{
		Attr:     "BlockingPostRun",
		Option:   "blocking_post_run",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/blocking_post_run"),
	}

	KWBlockingPostStart = keywords.Keyword{
		Attr:     "BlockingPostStart",
		Option:   "blocking_post_start",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/blocking_post_start"),
	}

	KWBlockingPostStop = keywords.Keyword{
		Attr:     "BlockingPostStop",
		Option:   "blocking_post_stop",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/blocking_post_stop"),
	}

	KWBlockingPostUnprovision = keywords.Keyword{
		Attr:     "BlockingPostUnprovision",
		Option:   "blocking_post_unprovision",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/blocking_post_unprovision"),
	}

	KWBlockingPreProvision = keywords.Keyword{
		Attr:     "BlockingPreProvision",
		Option:   "blocking_pre_provision",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/blocking_pre_provision"),
	}

	KWBlockingPreRun = keywords.Keyword{
		Attr:     "BlockingPreRun",
		Option:   "blocking_pre_run",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/blocking_pre_run"),
	}

	KWBlockingPreStart = keywords.Keyword{
		Attr:     "BlockingPreStart",
		Option:   "blocking_pre_start",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/blocking_pre_start"),
	}

	KWBlockingPreStop = keywords.Keyword{
		Attr:     "BlockingPreStop",
		Option:   "blocking_pre_stop",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/blocking_pre_stop"),
	}

	KWBlockingPreUnprovision = keywords.Keyword{
		Attr:     "BlockingPreUnprovision",
		Option:   "blocking_pre_unprovision",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/blocking_pre_unprovision"),
	}

	KWDisable = keywords.Keyword{
		Attr:      "Disable",
		Converter: "bool",
		Option:    "disable",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/disable"),
	}

	KWEnableProvision = keywords.Keyword{
		Attr:      "EnableProvision",
		Converter: "bool",
		Default:   "true",
		Option:    "provision",
		Text:      keywords.NewText(fs, "text/kw/provision"),
	}

	KWEnableUnprovision = keywords.Keyword{
		Attr:      "EnableUnprovision",
		Converter: "bool",
		Default:   "true",
		Option:    "unprovision",
		Text:      keywords.NewText(fs, "text/kw/unprovision"),
	}

	KWEncap = keywords.Keyword{
		Attr:      "Encap",
		Converter: "bool",
		Option:    "encap",
		Text:      keywords.NewText(fs, "text/kw/encap"),
	}

	KWMonitor = keywords.Keyword{
		Attr:      "Monitor",
		Converter: "bool",
		Option:    "monitor",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/monitor"),
	}

	KWOptional = keywords.Keyword{
		Attr:      "Optional",
		Converter: "bool",
		Inherit:   keywords.InheritHead2Leaf,
		Option:    "optional",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/optional"),
	}

	KWPostProvision = keywords.Keyword{
		Attr:     "PostProvision",
		Option:   "post_provision",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/post_provision"),
	}

	KWPostRun = keywords.Keyword{
		Attr:     "PostRun",
		Option:   "post_run",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/post_provision"),
	}

	KWPostStart = keywords.Keyword{
		Attr:     "PostStart",
		Option:   "post_start",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/post_provision"),
	}

	KWPostStop = keywords.Keyword{
		Attr:     "PostStop",
		Option:   "post_stop",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/post_provision"),
	}

	KWPostUnprovision = keywords.Keyword{
		Attr:     "PostUnprovision",
		Option:   "post_unprovision",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/post_provision"),
	}

	KWPreProvision = keywords.Keyword{
		Attr:     "PreProvision",
		Option:   "pre_provision",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/post_provision"),
	}

	KWPreRun = keywords.Keyword{
		Attr:     "PreRun",
		Option:   "pre_run",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/post_provision"),
	}

	KWPreStart = keywords.Keyword{
		Attr:     "PreStart",
		Option:   "pre_start",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/post_provision"),
	}

	KWPreStop = keywords.Keyword{
		Attr:     "PreStop",
		Option:   "pre_stop",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/post_provision"),
	}

	KWPreUnprovision = keywords.Keyword{
		Attr:     "PreUnprovision",
		Option:   "pre_unprovision",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/post_provision"),
	}

	KWProvisionRequires = keywords.Keyword{
		Attr:    "ProvisionRequires",
		Example: "ip#0 fs#0(down,stdby down)",
		Option:  "provision_requires",
		Text:    keywords.NewText(fs, "text/kw/provision_requires"),
	}

	KWRestart = keywords.Keyword{
		Attr:      "Restart.Count",
		Default:   "0",
		Converter: "int",
		Option:    "restart",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/restart"),
	}

	KWRestartDelay = keywords.Keyword{
		Attr:      "Restart.Delay",
		Converter: "duration",
		Default:   "500ms",
		Option:    "restart_delay",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/post_provision"),
	}

	KWRunRequires = keywords.Keyword{
		Attr:    "RunRequires",
		Example: "ip#0 fs#0(down,stdby down)",
		Option:  "run_requires",
		Text:    keywords.NewText(fs, "text/kw/run_requires"),
	}

	KWSCSIPersistentReservationEnabled = keywords.Keyword{
		Attr:      "SCSIPersistentReservation.Enabled",
		Converter: "bool",
		Option:    "scsireserv",
		Text:      keywords.NewText(fs, "text/kw/scsireserv"),
	}

	KWSCSIPersistentReservationKey = keywords.Keyword{
		Attr:     "SCSIPersistentReservation.Key",
		Option:   "prkey",
		Scopable: true,
		Default:  "{node.node.prkey}",
		Text:     keywords.NewText(fs, "text/kw/prkey"),
	}

	KWSCSIPersistentReservationNoPreemptAbort = keywords.Keyword{
		Attr:      "SCSIPersistentReservation.NoPreemptAbort",
		Converter: "bool",
		Option:    "no_preempt_abort",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/no_preempt_abort"),
	}

	KWShared = keywords.Keyword{
		Attr:      "Shared",
		Converter: "bool",
		Option:    "shared",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/shared"),
	}

	KWStandby = keywords.Keyword{
		Attr:      "Standby",
		Converter: "bool",
		Option:    "standby",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/standby"),
	}

	KWStartRequires = keywords.Keyword{
		Attr:    "StartRequires",
		Example: "ip#0 fs#0(down,stdby down)",
		Option:  "start_requires",
		Text:    keywords.NewText(fs, "text/kw/start_requires"),
	}

	KWStopRequires = keywords.Keyword{
		Attr:    "StopRequires",
		Example: "ip#0 fs#0(down,stdby down)",
		Option:  "stop_requires",
		Text:    keywords.NewText(fs, "text/kw/stop_requires"),
	}

	KWSubset = keywords.Keyword{
		Attr:     "Subset",
		Option:   "subset",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/post_provision"),
	}

	KWSyncRequires = keywords.Keyword{
		Attr:    "SyncRequires",
		Example: "ip#0 fs#0(down,stdby down)",
		Option:  "sync_requires",
		Text:    keywords.NewText(fs, "text/kw/sync_requires"),
	}

	KWTags = keywords.Keyword{
		Attr:      "Tags",
		Converter: "set",
		Option:    "tags",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/tags"),
	}

	KWUnprovisionRequires = keywords.Keyword{
		Attr:    "UnprovisionRequires",
		Example: "ip#0 fs#0(down,stdby down)",
		Option:  "unprovision_requires",
		Text:    keywords.NewText(fs, "text/kw/unprovision_requires"),
	}

	SCSIPersistentReservationKeywords = []keywords.Keyword{
		KWSCSIPersistentReservationEnabled,
		KWSCSIPersistentReservationKey,
		KWSCSIPersistentReservationNoPreemptAbort,
	}

	starterKeywords = []Attr{
		KWBlockingPostStart,
		KWBlockingPreStart,
		KWPostStart,
		KWPreStart,
		KWRestart,
		KWRestartDelay,
		KWStartRequires,
	}

	stopperKeywords = []Attr{
		KWBlockingPostStop,
		KWBlockingPreStop,
		KWPostStop,
		KWPreStop,
		KWStopRequires,
	}

	provisionerKeywords = []Attr{
		KWBlockingPostProvision,
		KWBlockingPreProvision,
		KWEnableProvision,
		KWPostProvision,
		KWPreProvision,
		KWProvisionRequires,
	}

	unprovisionerKeywords = []Attr{
		KWBlockingPostUnprovision,
		KWBlockingPreUnprovision,
		KWEnableUnprovision,
		KWPostUnprovision,
		KWPreUnprovision,
		KWUnprovisionRequires,
	}

	syncerKeywords = []Attr{
		KWOptional,
		KWSyncRequires,
	}

	runnerKeywords = []Attr{
		KWOptional,
		KWBlockingPostRun,
		KWBlockingPreRun,
		KWPostRun,
		KWPreRun,
		KWRunRequires,
	}

	genericKeywords = []Attr{
		KWDisable,
		KWEncap,
		KWMonitor,
		KWOptional,
		KWShared,
		KWStandby,
		KWSubset,
		KWTags,
	}
)
