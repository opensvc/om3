package commands

import (
	"time"
)

type (
	// OptsGlobal contains options accepted by all actions
	OptsGlobal struct {
		Color          string
		Debug          bool
		Format         string
		Info           bool
		Local          bool
		NodeSelector   string
		ObjectSelector string
		Quiet          bool
		Server         string
	}

	// OptsAsync contains options accepted by all actions having an orchestration
	OptsAsync struct {
		Watch bool
		Wait  bool
		Time  time.Duration
	}

	// OptsResourceSelector contains options needed to initialize a
	// resourceselector.Options struct
	OptsResourceSelector struct {
		RID    string
		Subset string
		Tag    string
	}

	// OptsLock contains options accepted by all actions using an action lock
	OptsLock struct {
		Disable bool
		Timeout time.Duration
	}

	// OpTo sets a barrier when iterating over a resource lister
	OptTo struct {
		To     string
		UpTo   string // Deprecated
		DownTo string // Deprecated
	}
)
