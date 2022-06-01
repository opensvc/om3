package manifest

import (
	"context"

	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/util/converters"
)

type (
	//
	// T describes a driver so callers can format the input as the
	// driver expects.
	//
	// A typical allocation is:
	// m := New("fs", "flag").AddKeyword(kws...).AddContext(ctx...)
	//
	T struct {
		Group    driver.Group       `json:"group"`
		Name     string             `json:"name"`
		Keywords []keywords.Keyword `json:"keywords"`
		Context  []Context          `json:"context"`
	}

	//
	// Context is a key-value the resource expects to find in the input,
	// merged with keywords coming from configuration file.
	//
	// For example, a driver often needs the parent object Path, which
	// can be asked via:
	//
	// T{
	//     Context: []Context{
	//         {
	//             Key: "path",
	//             Ref:"object.path",
	//         },
	//     },
	// }
	//
	Context struct {
		// Key is the name of the key in the json representation of the context.
		Key string

		// Attr is the name of the field in the resource struct.
		Attr string

		// Ref is the code describing what context information to embed in the resource struct.
		Ref string
	}
)

type (
	provisioner interface {
		Provision(context.Context) error
	}
	unprovisioner interface {
		Unprovision(context.Context) error
	}
	starter interface {
		Start(context.Context) error
	}
	stopper interface {
		Stop(context.Context) error
	}
	runner interface {
		Run(context.Context) error
	}
	syncer interface {
		Sync(context.Context) error
	}
)

func (t *T) AddInterfacesKeywords(r interface{}) *T {
	if _, ok := r.(starter); ok {
		t.AddKeyword(keywords.Keyword{
			Option:  "start_requires",
			Attr:    "StartRequires",
			Example: "ip#0 fs#0(down,stdby down)",
			Text:    "A whitespace-separated list of conditions to meet to accept doing a 'start' action. A condition is expressed as ``<rid>(<state>,...)``. If states are omitted, ``up,stdby up`` is used as the default expected states.",
		})
	}
	if _, ok := r.(stopper); ok {
		t.AddKeyword(keywords.Keyword{
			Option:  "stop_requires",
			Attr:    "StopRequires",
			Example: "ip#0 fs#0(down,stdby down)",
			Text:    "A whitespace-separated list of conditions to meet to accept doing a 'stop' action. A condition is expressed as ``<rid>(<state>,...)``. If states are omitted, ``up,stdby up`` is used as the default expected states.",
		})
	}
	if _, ok := r.(provisioner); ok {
		t.AddKeyword(keywords.Keyword{
			Option:  "provision_requires",
			Attr:    "ProvisionRequires",
			Example: "ip#0 fs#0(down,stdby down)",
			Text:    "A whitespace-separated list of conditions to meet to accept doing a 'start' action. A condition is expressed as ``<rid>(<state>,...)``. If states are omitted, ``up,stdby up`` is used as the default expected states.",
		})
	}
	if _, ok := r.(unprovisioner); ok {
		t.AddKeyword(keywords.Keyword{
			Option:  "unprovision_requires",
			Attr:    "UnprovisionRequires",
			Example: "ip#0 fs#0(down,stdby down)",
			Text:    "A whitespace-separated list of conditions to meet to accept doing a 'unprovision' action. A condition is expressed as ``<rid>(<state>,...)``. If states are omitted, ``up,stdby up`` is used as the default expected states.",
		})
	}
	if _, ok := r.(syncer); ok {
		t.AddKeyword(keywords.Keyword{
			Option:  "sync_requires",
			Attr:    "SyncRequires",
			Example: "ip#0 fs#0(down,stdby down)",
			Text:    "A whitespace-separated list of conditions to meet to accept doing a 'sync' action. A condition is expressed as ``<rid>(<state>,...)``. If states are omitted, ``up,stdby up`` is used as the default expected states.",
		})
	}
	if _, ok := r.(runner); ok {
		t.AddKeyword(keywords.Keyword{
			Option:  "run_requires",
			Attr:    "RunRequires",
			Example: "ip#0 fs#0(down,stdby down)",
			Text:    "A whitespace-separated list of conditions to meet to accept doing a 'run' action. A condition is expressed as ``<rid>(<state>,...)``. If states are omitted, ``up,stdby up`` is used as the default expected states.",
		})
	}
	return t
}

var genericKeywords = []keywords.Keyword{
	{
		Option:    "disable",
		Attr:      "Disable",
		Scopable:  true,
		Converter: converters.Bool,
		Text:      "A disabled resource will be ignored on service startup and shutdown. Its status is always reported ``n/a``.\n\nSet in DEFAULT, the whole service is disabled. A disabled service does not honor :c-action:`start` and :c-action:`stop` actions. These actions immediately return success.\n\n:cmd:`om <path> disable` only sets :kw:`DEFAULT.disable`. As resources disabled state is not changed, :cmd:`om <path> enable` does not enable disabled resources.",
	},
	{
		Option:    "optional",
		Attr:      "Optional",
		Scopable:  true,
		Converter: converters.Bool,
		Inherit:   keywords.InheritHead2Leaf,
		Text:      "Action failures on optional resources are logged but do not stop the action sequence. Also the optional resource status is not aggregated to the instance 'availstatus', but aggregated to the 'overallstatus'. Resource tagged :c-tag:`noaction` and sync resources are automatically considered optional. Useful for resources like dump filesystems for example.",
	},
	{
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
	},
	{
		Option:    "monitor",
		Attr:      "Monitor",
		Scopable:  true,
		Converter: converters.Bool,
		Text:      "A down monitored resource will trigger a the monitor action (crash or reboot the node, freezestop or switch the service) if the monitor thinks the resource should be up and it all restart tries failed.",
	},
	{
		Option:    "shared",
		Attr:      "Shared",
		Scopable:  true,
		Converter: converters.Bool,
		Text:      "Set to ``true`` to skip the resource on provision and unprovision actions if the action has already been done by a peer. Shared resources, like vg built on SAN disks must be provisioned once. All resources depending on a shared resource must also be flagged as shared.",
	},
	{
		Option:    "standby",
		Attr:      "Standby",
		Scopable:  true,
		Converter: converters.Bool,
		Text:      "Always start the resource, even on standby instances. The daemon is responsible for starting standby resources. A resource can be set standby on a subset of nodes using keyword scoping.\n\nA typical use-case is sync'ed fs on non-shared disks: the remote fs must be mounted to not overflow the underlying fs.\n\n.. warning:: Don't set shared resources standby: fs on shared disks for example.",
	},
	{
		Option:    "tags",
		Attr:      "Tags",
		Scopable:  true,
		Converter: converters.Set,
		Text:      "A list of tags. Arbitrary tags can be used to limit action scope to resources with a specific tag. Some tags can influence the driver behaviour. For example :c-tag:`noaction` avoids any state changing action from the driver and implies ``optional=true``, :c-tag:`nostatus` forces the status to n/a.",
	},
	{
		Option:   "subset",
		Attr:     "Subset",
		Scopable: true,
		Text:     "Assign the resource to a specific subset.",
	},
	{
		Option:   "blocking_pre_start",
		Attr:     "BlockingPreStart",
		Scopable: true,
		Text:     "A command or script to execute before the resource :c-action:`start` action. Errors interrupt the action.",
	},
	{
		Option:   "blocking_pre_stop",
		Attr:     "BlockingPreStop",
		Scopable: true,
		Text:     "A command or script to execute before the resource :c-action:`stop` action. Errors interrupt the action.",
	},
	{
		Option:   "pre_start",
		Attr:     "PreStart",
		Scopable: true,
		Text:     "A command or script to execute before the resource :c-action:`start` action. Errors do not interrupt the action.",
	},
	{
		Option:   "pre_stop",
		Attr:     "PreStop",
		Scopable: true,
		Text:     "A command or script to execute before the resource :c-action:`stop` action. Errors do not interrupt the action.",
	},
	{
		Option:   "blocking_post_start",
		Attr:     "BlockingPostStart",
		Scopable: true,
		Text:     "A command or script to execute after the resource :c-action:`start` action. Errors interrupt the action.",
	},
	{
		Option:   "blocking_post_stop",
		Attr:     "BlockingPostStop",
		Scopable: true,
		Text:     "A command or script to execute after the resource :c-action:`stop` action. Errors interrupt the action.",
	},
	{
		Option:   "post_start",
		Attr:     "PostStart",
		Scopable: true,
		Text:     "A command or script to execute after the resource :c-action:`start` action. Errors do not interrupt the action.",
	},
	{
		Option:   "post_stop",
		Attr:     "PostStop",
		Scopable: true,
		Text:     "A command or script to execute after the resource :c-action:`stop` action. Errors do not interrupt the action.",
	},
}

var RunTriggerKeywords = []keywords.Keyword{
	{
		Option:   "blocking_pre_run",
		Attr:     "BlockingPreRun",
		Scopable: true,
		Text:     "A command or script to execute before the resource :c-action:`run` action. Errors interrupt the action.",
	},
	{
		Option:   "blocking_post_run",
		Attr:     "BlockingPostRun",
		Scopable: true,
		Text:     "A command or script to execute after the resource :c-action:`run` action. Errors interrupt the action.",
	},
	{
		Option:   "pre_run",
		Attr:     "PreRun",
		Scopable: true,
		Text:     "A command or script to execute before the resource :c-action:`run` action. Errors do not interrupt the action.",
	},
	{
		Option:   "post_run",
		Attr:     "PostRun",
		Scopable: true,
		Text:     "A command or script to execute after the resource :c-action:`run` action. Errors do not interrupt the action.",
	},
}

var ProvisioningKeywords = []keywords.Keyword{
	{
		Option:    "provision",
		Attr:      "EnableProvision",
		Converter: converters.Bool,
		Default:   "true",
		Text:      "Set to false to skip the resource on provision and unprovision actions. Warning: Provision implies destructive operations like formating. Unprovision destroys service data.",
	},
	{
		Option:    "unprovision",
		Attr:      "EnableUnprovision",
		Converter: converters.Bool,
		Default:   "true",
		Text:      "Set to false to skip the resource on unprovision actions. Warning: Unprovision destroys service data.",
	},
	{
		Option:   "blocking_pre_provision",
		Attr:     "BlockingPreProvision",
		Scopable: true,
		Text:     "A command or script to execute before the resource :c-action:`provision` action. Errors interrupt the action.",
	},
	{
		Option:   "blocking_post_provision",
		Attr:     "BlockingPostProvision",
		Scopable: true,
		Text:     "A command or script to execute after the resource :c-action:`provision` action. Errors interrupt the action.",
	},
	{
		Option:   "pre_provision",
		Attr:     "PreProvision",
		Scopable: true,
		Text:     "A command or script to execute before the resource :c-action:`provision` action. Errors do not interrupt the action.",
	},
	{
		Option:   "post_provision",
		Attr:     "PostProvision",
		Scopable: true,
		Text:     "A command or script to execute after the resource :c-action:`provision` action. Errors do not interrupt the action.",
	},
	{
		Option:   "blocking_pre_unprovision",
		Attr:     "BlockingPreUnprovision",
		Scopable: true,
		Text:     "A command or script to execute before the resource :c-action:`unprovision` action. Errors interrupt the action.",
	},
	{
		Option:   "blocking_post_unprovision",
		Attr:     "BlockingPostUnprovision",
		Scopable: true,
		Text:     "A command or script to execute after the resource :c-action:`unprovision` action. Errors interrupt the action.",
	},
	{
		Option:   "pre_unprovision",
		Attr:     "PreUnprovision",
		Scopable: true,
		Text:     "A command or script to execute before the resource :c-action:`unprovision` action. Errors do not interrupt the action.",
	},
	{
		Option:   "post_unprovision",
		Attr:     "PostUnprovision",
		Scopable: true,
		Text:     "A command or script to execute after the resource :c-action:`unprovision` action. Errors do not interrupt the action.",
	},
}

func New(group driver.Group, name string, r interface{}) *T {
	t := &T{
		Group: group,
		Name:  name,
	}
	t.AddKeyword(genericKeywords...)
	t.AddInterfacesKeywords(r)
	return t
}

func (t *T) AddKeyword(kws ...keywords.Keyword) *T {
	t.Keywords = append(t.Keywords, kws...)
	return t
}

func (t *T) AddContext(ctx ...Context) *T {
	t.Context = append(t.Context, ctx...)
	return t
}
