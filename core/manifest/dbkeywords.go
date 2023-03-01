package manifest

import (
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/util/converters"
)

var (
	KWBlockingPostProvision = keywords.Keyword{
		Option:   "blocking_post_provision",
		Attr:     "BlockingPostProvision",
		Scopable: true,
		Text:     "A command or script to execute after the resource :c-action:`provision` action. Errors interrupt the action.",
	}

	KWBlockingPostRun = keywords.Keyword{
		Option:   "blocking_post_run",
		Attr:     "BlockingPostRun",
		Scopable: true,
		Text:     "A command or script to execute after the resource :c-action:`run` action. Errors interrupt the action.",
	}

	KWBlockingPostStart = keywords.Keyword{
		Option:   "blocking_post_start",
		Attr:     "BlockingPostStart",
		Scopable: true,
		Text:     "A command or script to execute after the resource :c-action:`start` action. Errors interrupt the action.",
	}

	KWBlockingPostStop = keywords.Keyword{
		Option:   "blocking_post_stop",
		Attr:     "BlockingPostStop",
		Scopable: true,
		Text:     "A command or script to execute after the resource :c-action:`stop` action. Errors interrupt the action.",
	}

	KWBlockingPostUnprovision = keywords.Keyword{
		Option:   "blocking_post_unprovision",
		Attr:     "BlockingPostUnprovision",
		Scopable: true,
		Text:     "A command or script to execute after the resource :c-action:`unprovision` action. Errors interrupt the action.",
	}

	KWBlockingPreProvision = keywords.Keyword{
		Option:   "blocking_pre_provision",
		Attr:     "BlockingPreProvision",
		Scopable: true,
		Text:     "A command or script to execute before the resource :c-action:`provision` action. Errors interrupt the action.",
	}

	KWBlockingPreRun = keywords.Keyword{
		Option:   "blocking_pre_run",
		Attr:     "BlockingPreRun",
		Scopable: true,
		Text:     "A command or script to execute before the resource :c-action:`run` action. Errors interrupt the action.",
	}

	KWBlockingPreStart = keywords.Keyword{
		Option:   "blocking_pre_start",
		Attr:     "BlockingPreStart",
		Scopable: true,
		Text:     "A command or script to execute before the resource :c-action:`start` action. Errors interrupt the action.",
	}

	KWBlockingPreStop = keywords.Keyword{
		Option:   "blocking_pre_stop",
		Attr:     "BlockingPreStop",
		Scopable: true,
		Text:     "A command or script to execute before the resource :c-action:`stop` action. Errors interrupt the action.",
	}

	KWBlockingPreUnprovision = keywords.Keyword{
		Option:   "blocking_pre_unprovision",
		Attr:     "BlockingPreUnprovision",
		Scopable: true,
		Text:     "A command or script to execute before the resource :c-action:`unprovision` action. Errors interrupt the action.",
	}

	KWDisable = keywords.Keyword{
		Option:    "disable",
		Attr:      "Disable",
		Scopable:  true,
		Converter: converters.Bool,
		Text:      "A disabled resource will be ignored on service startup and shutdown. Its status is always reported ``n/a``.\n\nSet in DEFAULT, the whole service is disabled. A disabled service does not honor :c-action:`start` and :c-action:`stop` actions. These actions immediately return success.\n\n:cmd:`om <path> disable` only sets :kw:`DEFAULT.disable`. As resources disabled state is not changed, :cmd:`om <path> enable` does not enable disabled resources.",
	}

	KWEnableProvision = keywords.Keyword{
		Option:    "provision",
		Attr:      "EnableProvision",
		Converter: converters.Bool,
		Default:   "true",
		Text:      "Set to false to skip the resource on provision and unprovision actions. Warning: Provision implies destructive operations like formating. Unprovision destroys service data.",
	}

	KWEnableUnprovision = keywords.Keyword{
		Option:    "unprovision",
		Attr:      "EnableUnprovision",
		Converter: converters.Bool,
		Default:   "true",
		Text:      "Set to false to skip the resource on unprovision actions. Warning: Unprovision destroys service data.",
	}

	KWMonitor = keywords.Keyword{
		Option:    "monitor",
		Attr:      "Monitor",
		Scopable:  true,
		Converter: converters.Bool,
		Text:      "A down monitored resource will trigger a the monitor action (crash or reboot the node, freezestop or switch the service) if the monitor thinks the resource should be up and it all restart tries failed.",
	}

	KWOptional = keywords.Keyword{
		Option:    "optional",
		Attr:      "Optional",
		Scopable:  true,
		Converter: converters.Bool,
		Inherit:   keywords.InheritHead2Leaf,
		Text:      "Action failures on optional resources are logged but do not stop the action sequence. Also the optional resource status is not aggregated to the instance 'availstatus', but aggregated to the 'overallstatus'. Resource tagged :c-tag:`noaction` and sync resources are automatically considered optional. Useful for resources like dump filesystems for example.",
	}

	KWOptionalTrue = keywords.Keyword{
		Option:    "optional",
		Attr:      "Optional",
		Scopable:  true,
		Converter: converters.Bool,
		Inherit:   keywords.InheritHead2Leaf,
		Default:   "true",
		Text:      "Action failures on optional resources are logged but do not stop the action sequence. Also the optional resource status is not aggregated to the instance 'availstatus', but aggregated to the 'overallstatus'. Resource tagged :c-tag:`noaction` and sync resources are automatically considered optional. Useful for resources like dump filesystems for example.",
	}

	KWPostProvision = keywords.Keyword{
		Option:   "post_provision",
		Attr:     "PostProvision",
		Scopable: true,
		Text:     "A command or script to execute after the resource :c-action:`provision` action. Errors do not interrupt the action.",
	}

	KWPostRun = keywords.Keyword{
		Option:   "post_run",
		Attr:     "PostRun",
		Scopable: true,
		Text:     "A command or script to execute after the resource :c-action:`run` action. Errors do not interrupt the action.",
	}

	KWPostStart = keywords.Keyword{
		Option:   "post_start",
		Attr:     "PostStart",
		Scopable: true,
		Text:     "A command or script to execute after the resource :c-action:`start` action. Errors do not interrupt the action.",
	}

	KWPostStop = keywords.Keyword{
		Option:   "post_stop",
		Attr:     "PostStop",
		Scopable: true,
		Text:     "A command or script to execute after the resource :c-action:`stop` action. Errors do not interrupt the action.",
	}

	KWPostUnprovision = keywords.Keyword{
		Option:   "post_unprovision",
		Attr:     "PostUnprovision",
		Scopable: true,
		Text:     "A command or script to execute after the resource :c-action:`unprovision` action. Errors do not interrupt the action.",
	}

	KWPreProvision = keywords.Keyword{
		Option:   "pre_provision",
		Attr:     "PreProvision",
		Scopable: true,
		Text:     "A command or script to execute before the resource :c-action:`provision` action. Errors do not interrupt the action.",
	}

	KWPreRun = keywords.Keyword{
		Option:   "pre_run",
		Attr:     "PreRun",
		Scopable: true,
		Text:     "A command or script to execute before the resource :c-action:`run` action. Errors do not interrupt the action.",
	}

	KWPreStart = keywords.Keyword{
		Option:   "pre_start",
		Attr:     "PreStart",
		Scopable: true,
		Text:     "A command or script to execute before the resource :c-action:`start` action. Errors do not interrupt the action.",
	}

	KWPreStop = keywords.Keyword{
		Option:   "pre_stop",
		Attr:     "PreStop",
		Scopable: true,
		Text:     "A command or script to execute before the resource :c-action:`stop` action. Errors do not interrupt the action.",
	}

	KWPreUnprovision = keywords.Keyword{
		Option:   "pre_unprovision",
		Attr:     "PreUnprovision",
		Scopable: true,
		Text:     "A command or script to execute before the resource :c-action:`unprovision` action. Errors do not interrupt the action.",
	}

	KWProvisionRequires = keywords.Keyword{
		Option:  "provision_requires",
		Attr:    "ProvisionRequires",
		Example: "ip#0 fs#0(down,stdby down)",
		Text:    "A whitespace-separated list of conditions to meet to accept doing a 'start' action. A condition is expressed as ``<rid>(<state>,...)``. If states are omitted, ``up,stdby up`` is used as the default expected states.",
	}

	KWRestart = keywords.Keyword{
		Option:    "restart",
		Attr:      "Restart",
		Scopable:  true,
		Converter: converters.Int,
		Default:   "0",
		Text: "The agent will try to restart a resource <n> times before falling back to the monitor action. A resource restart is triggered if:" +
			"the resource is not disabled and its status is not up, " +
			"and the node is not frozen, " +
			"and the service instance is not frozen " +
			"and its local expect is set to ``started``. " +
			"If a resource has a restart set to a value greater than zero, its status is evaluated " +
			"at the frequency defined by :kw:`DEFAULT.monitor_schedule` " +
			"instead of the frequency defined by :kw:`DEFAULT.status_schedule`. " +
			":kw:`restart_delay` defines the interval between two restarts. " +
			"Standby resources have a particular value to ensure best effort to restart standby resources, " +
			"default value is 2, and value lower than 2 are changed to 2.",
	}

	KWRestartDelay = keywords.Keyword{
		Option:    "restart_delay",
		Attr:      "RestartDelay",
		Scopable:  true,
		Converter: converters.Duration,
		Default:   "500ms",
		Text: "Define minimum delay between two triggered restarts of a same resource (used when :kw:`restart`is defined). " +
			"Default value is 0 (no delay).",
	}

	KWRunRequires = keywords.Keyword{
		Option:  "run_requires",
		Attr:    "RunRequires",
		Example: "ip#0 fs#0(down,stdby down)",
		Text:    "A whitespace-separated list of conditions to meet to accept doing a 'run' action. A condition is expressed as ``<rid>(<state>,...)``. If states are omitted, ``up,stdby up`` is used as the default expected states.",
	}

	KWSCSIPersistentReservationEnabled = keywords.Keyword{
		Option:    "scsireserv",
		Attr:      "SCSIPersistentReservation.Enabled",
		Converter: converters.Bool,
		Text:      "If set to ``true``, OpenSVC will try to acquire a type-5 (write exclusive, registrant only) scsi3 persistent reservation on every path to every disks held by this resource. Existing reservations are preempted to not block service start-up. If the start-up was not legitimate the data are still protected from being written over from both nodes. If set to ``false`` or not set, :kw:`scsireserv` can be activated on a per-resource basis.",
	}

	KWSCSIPersistentReservationKey = keywords.Keyword{
		Option:   "prkey",
		Attr:     "SCSIPersistentReservation.Key",
		Scopable: true,
		Text:     "Defines a specific persistent reservation key for the resource. Takes priority over the service-level defined prkey and the node.conf specified prkey.",
	}

	KWSCSIPersistentReservationNoPreemptAbort = keywords.Keyword{
		Option:    "no_preempt_abort",
		Attr:      "SCSIPersistentReservation.NoPreemptAbort",
		Scopable:  true,
		Converter: converters.Bool,
		Text:      "If set to ``true``, OpenSVC will preempt scsi reservation with a preempt command instead of a preempt and and abort. Some scsi target implementations do not support this last mode (esx). If set to ``false`` or not set, :kw:`no_preempt_abort` can be activated on a per-resource basis.",
	}

	KWShared = keywords.Keyword{
		Option:    "shared",
		Attr:      "Shared",
		Scopable:  true,
		Converter: converters.Bool,
		Text:      "Set to ``true`` to skip the resource on provision and unprovision actions if the action has already been done by a peer. Shared resources, like vg built on SAN disks must be provisioned once. All resources depending on a shared resource must also be flagged as shared.",
	}

	KWStandby = keywords.Keyword{
		Option:    "standby",
		Attr:      "Standby",
		Scopable:  true,
		Converter: converters.Bool,
		Text:      "Always start the resource, even on standby instances. The daemon is responsible for starting standby resources. A resource can be set standby on a subset of nodes using keyword scoping.\n\nA typical use-case is sync'ed fs on non-shared disks: the remote fs must be mounted to not overflow the underlying fs.\n\n.. warning:: Don't set shared resources standby: fs on shared disks for example.",
	}

	KWStartRequires = keywords.Keyword{
		Option:  "start_requires",
		Attr:    "StartRequires",
		Example: "ip#0 fs#0(down,stdby down)",
		Text:    "A whitespace-separated list of conditions to meet to accept doing a 'start' action. A condition is expressed as ``<rid>(<state>,...)``. If states are omitted, ``up,stdby up`` is used as the default expected states.",
	}

	KWStopRequires = keywords.Keyword{
		Option:  "stop_requires",
		Attr:    "StopRequires",
		Example: "ip#0 fs#0(down,stdby down)",
		Text:    "A whitespace-separated list of conditions to meet to accept doing a 'stop' action. A condition is expressed as ``<rid>(<state>,...)``. If states are omitted, ``up,stdby up`` is used as the default expected states.",
	}

	KWSubset = keywords.Keyword{
		Option:   "subset",
		Attr:     "Subset",
		Scopable: true,
		Text:     "Assign the resource to a specific subset.",
	}

	KWSyncRequires = keywords.Keyword{
		Option:  "sync_requires",
		Attr:    "SyncRequires",
		Example: "ip#0 fs#0(down,stdby down)",
		Text:    "A whitespace-separated list of conditions to meet to accept doing a 'sync' action. A condition is expressed as ``<rid>(<state>,...)``. If states are omitted, ``up,stdby up`` is used as the default expected states.",
	}

	KWTags = keywords.Keyword{
		Option:    "tags",
		Attr:      "Tags",
		Scopable:  true,
		Converter: converters.Set,
		Text:      "A list of tags. Arbitrary tags can be used to limit action scope to resources with a specific tag. Some tags can influence the driver behaviour. For example :c-tag:`noaction` avoids any state changing action from the driver and implies ``optional=true``, :c-tag:`nostatus` forces the status to n/a.",
	}

	KWUnprovisionRequires = keywords.Keyword{
		Option:  "unprovision_requires",
		Attr:    "UnprovisionRequires",
		Example: "ip#0 fs#0(down,stdby down)",
		Text:    "A whitespace-separated list of conditions to meet to accept doing a 'unprovision' action. A condition is expressed as ``<rid>(<state>,...)``. If states are omitted, ``up,stdby up`` is used as the default expected states.",
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
