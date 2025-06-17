package commoncmd

import "time"

type (
	// OptsAsync contains options accepted by all actions having an orchestration
	OptsAsync struct {
		Watch bool
		Wait  bool
		Time  time.Duration
	}

	// OptsLogs contains options used by all log commands:
	// node logs, cluster logs, object logs
	OptsLogs struct {
		Follow bool
		Lines  int
		Filter []string
	}

	// OptsResourceSelector contains options needed to initialize a
	// resourceselector.Options struct
	OptsResourceSelector struct {
		RID    string
		Subset string
		Tag    string
	}

	// OptsEncap contains options to select where a instance action runs
	// with a encap enabled service
	OptsEncap struct {
		// A list of encap instance, identified by hostname of the container
		Slaves []string

		// The host instance
		Master bool

		// All encap instances
		AllSlaves bool
	}

	// OptsLock contains options accepted by all actions using an action lock
	OptsLock struct {
		Disable bool
		Timeout time.Duration
	}

	// OptTo sets a barrier when iterating over a resource lister
	OptTo struct {
		To     string
		UpTo   string // Deprecated
		DownTo string // Deprecated
	}
)
