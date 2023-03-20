package manifest

import (
	"embed"

	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/util/converters"
)

//go:embed text
var fs embed.FS

var (
	KWBlockingPostProvision = keywords.Keyword{
		Option:   "blocking_post_provision",
		Attr:     "BlockingPostProvision",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/blocking_post_provision"),
	}

	KWBlockingPostRun = keywords.Keyword{
		Option:   "blocking_post_run",
		Attr:     "BlockingPostRun",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/blocking_post_run"),
	}

	KWBlockingPostStart = keywords.Keyword{
		Option:   "blocking_post_start",
		Attr:     "BlockingPostStart",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/blocking_post_start"),
	}

	KWBlockingPostStop = keywords.Keyword{
		Option:   "blocking_post_stop",
		Attr:     "BlockingPostStop",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/blocking_post_stop"),
	}

	KWBlockingPostUnprovision = keywords.Keyword{
		Option:   "blocking_post_unprovision",
		Attr:     "BlockingPostUnprovision",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/blocking_post_unprovision"),
	}

	KWBlockingPreProvision = keywords.Keyword{
		Option:   "blocking_pre_provision",
		Attr:     "BlockingPreProvision",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/blocking_pre_provision"),
	}

	KWBlockingPreRun = keywords.Keyword{
		Option:   "blocking_pre_run",
		Attr:     "BlockingPreRun",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/blocking_pre_run"),
	}

	KWBlockingPreStart = keywords.Keyword{
		Option:   "blocking_pre_start",
		Attr:     "BlockingPreStart",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/blocking_pre_start"),
	}

	KWBlockingPreStop = keywords.Keyword{
		Option:   "blocking_pre_stop",
		Attr:     "BlockingPreStop",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/blocking_pre_stop"),
	}

	KWBlockingPreUnprovision = keywords.Keyword{
		Option:   "blocking_pre_unprovision",
		Attr:     "BlockingPreUnprovision",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/blocking_pre_unprovision"),
	}

	KWDisable = keywords.Keyword{
		Option:    "disable",
		Attr:      "Disable",
		Scopable:  true,
		Converter: converters.Bool,
		Text:      keywords.NewText(fs, "text/kw/disable"),
	}

	KWEnableProvision = keywords.Keyword{
		Option:    "provision",
		Attr:      "EnableProvision",
		Converter: converters.Bool,
		Default:   "true",
		Text:      keywords.NewText(fs, "text/kw/provision"),
	}

	KWEnableUnprovision = keywords.Keyword{
		Option:    "unprovision",
		Attr:      "EnableUnprovision",
		Converter: converters.Bool,
		Default:   "true",
		Text:      keywords.NewText(fs, "text/kw/unprovision"),
	}

	KWMonitor = keywords.Keyword{
		Option:    "monitor",
		Attr:      "Monitor",
		Scopable:  true,
		Converter: converters.Bool,
		Text:      keywords.NewText(fs, "text/kw/monitor"),
	}

	KWOptional = keywords.Keyword{
		Option:    "optional",
		Attr:      "Optional",
		Scopable:  true,
		Converter: converters.Bool,
		Inherit:   keywords.InheritHead2Leaf,
		Text:      keywords.NewText(fs, "text/kw/optional"),
	}

	KWOptionalTrue = keywords.Keyword{
		Option:    "optional",
		Attr:      "Optional",
		Scopable:  true,
		Converter: converters.Bool,
		Inherit:   keywords.InheritHead2Leaf,
		Default:   "true",
		Text:      keywords.NewText(fs, "text/kw/optional"),
	}

	KWPostProvision = keywords.Keyword{
		Option:   "post_provision",
		Attr:     "PostProvision",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/post_provision"),
	}

	KWPostRun = keywords.Keyword{
		Option:   "post_run",
		Attr:     "PostRun",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/post_provision"),
	}

	KWPostStart = keywords.Keyword{
		Option:   "post_start",
		Attr:     "PostStart",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/post_provision"),
	}

	KWPostStop = keywords.Keyword{
		Option:   "post_stop",
		Attr:     "PostStop",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/post_provision"),
	}

	KWPostUnprovision = keywords.Keyword{
		Option:   "post_unprovision",
		Attr:     "PostUnprovision",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/post_provision"),
	}

	KWPreProvision = keywords.Keyword{
		Option:   "pre_provision",
		Attr:     "PreProvision",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/post_provision"),
	}

	KWPreRun = keywords.Keyword{
		Option:   "pre_run",
		Attr:     "PreRun",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/post_provision"),
	}

	KWPreStart = keywords.Keyword{
		Option:   "pre_start",
		Attr:     "PreStart",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/post_provision"),
	}

	KWPreStop = keywords.Keyword{
		Option:   "pre_stop",
		Attr:     "PreStop",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/post_provision"),
	}

	KWPreUnprovision = keywords.Keyword{
		Option:   "pre_unprovision",
		Attr:     "PreUnprovision",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/post_provision"),
	}

	KWProvisionRequires = keywords.Keyword{
		Option:  "provision_requires",
		Attr:    "ProvisionRequires",
		Example: "ip#0 fs#0(down,stdby down)",
		Text:    keywords.NewText(fs, "text/kw/provision_requires"),
	}

	KWRestart = keywords.Keyword{
		Option:    "restart",
		Attr:      "Restart",
		Scopable:  true,
		Converter: converters.Int,
		Default:   "0",
		Text:      keywords.NewText(fs, "text/kw/restart"),
	}

	KWRestartDelay = keywords.Keyword{
		Option:    "restart_delay",
		Attr:      "RestartDelay",
		Scopable:  true,
		Converter: converters.Duration,
		Default:   "500ms",
		Text:      keywords.NewText(fs, "text/kw/post_provision"),
	}

	KWRunRequires = keywords.Keyword{
		Option:  "run_requires",
		Attr:    "RunRequires",
		Example: "ip#0 fs#0(down,stdby down)",
		Text:    keywords.NewText(fs, "text/kw/run_requires"),
	}

	KWSCSIPersistentReservationEnabled = keywords.Keyword{
		Option:    "scsireserv",
		Attr:      "SCSIPersistentReservation.Enabled",
		Converter: converters.Bool,
		Text:      keywords.NewText(fs, "text/kw/scsireserv"),
	}

	KWSCSIPersistentReservationKey = keywords.Keyword{
		Option:   "prkey",
		Attr:     "SCSIPersistentReservation.Key",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/prkey"),
	}

	KWSCSIPersistentReservationNoPreemptAbort = keywords.Keyword{
		Option:    "no_preempt_abort",
		Attr:      "SCSIPersistentReservation.NoPreemptAbort",
		Scopable:  true,
		Converter: converters.Bool,
		Text:      keywords.NewText(fs, "text/kw/no_preempt_abort"),
	}

	KWShared = keywords.Keyword{
		Option:    "shared",
		Attr:      "Shared",
		Scopable:  true,
		Converter: converters.Bool,
		Text:      keywords.NewText(fs, "text/kw/shared"),
	}

	KWStandby = keywords.Keyword{
		Option:    "standby",
		Attr:      "Standby",
		Scopable:  true,
		Converter: converters.Bool,
		Text:      keywords.NewText(fs, "text/kw/standby"),
	}

	KWStartRequires = keywords.Keyword{
		Option:  "start_requires",
		Attr:    "StartRequires",
		Example: "ip#0 fs#0(down,stdby down)",
		Text:    keywords.NewText(fs, "text/kw/start_requires"),
	}

	KWStopRequires = keywords.Keyword{
		Option:  "stop_requires",
		Attr:    "StopRequires",
		Example: "ip#0 fs#0(down,stdby down)",
		Text:    keywords.NewText(fs, "text/kw/stop_requires"),
	}

	KWSubset = keywords.Keyword{
		Option:   "subset",
		Attr:     "Subset",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/post_provision"),
	}

	KWSyncRequires = keywords.Keyword{
		Option:  "sync_requires",
		Attr:    "SyncRequires",
		Example: "ip#0 fs#0(down,stdby down)",
		Text:    keywords.NewText(fs, "text/kw/sync_requires"),
	}

	KWTags = keywords.Keyword{
		Option:    "tags",
		Attr:      "Tags",
		Scopable:  true,
		Converter: converters.Set,
		Text:      keywords.NewText(fs, "text/kw/tags"),
	}

	KWUnprovisionRequires = keywords.Keyword{
		Option:  "unprovision_requires",
		Attr:    "UnprovisionRequires",
		Example: "ip#0 fs#0(down,stdby down)",
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
		KWOptionalTrue,
		KWSyncRequires,
	}

	runnerKeywords = []Attr{
		KWOptionalTrue,
		KWBlockingPostRun,
		KWBlockingPreRun,
		KWPostRun,
		KWPreRun,
		KWRunRequires,
	}

	genericKeywords = []Attr{
		KWDisable,
		KWMonitor,
		KWOptional,
		KWShared,
		KWStandby,
		KWSubset,
		KWTags,
	}
)
