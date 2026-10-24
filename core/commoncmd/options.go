package commoncmd

import "time"

type (
	// OptsAsync contains options accepted by all actions having an orchestration
	OptsAsync struct {
		Watch  bool
		Wait   bool
		Time   time.Duration
		Follow bool
	}

	// OptsLogs contains options used by all log commands:
	// node logs, cluster logs, object logs
	OptsLogs struct {
		Follow bool
		Lines  int
		Filter []string
		Grep   *string
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

		// TimeoutSet says --waitlock was given, and Timeout is not its
		// default: an action that would wait longer by default keeps to
		// it.
		TimeoutSet bool
	}

	// OptsRestore contains the options of a sync restore.
	OptsRestore struct {
		// Snapshot is the snapshot to restore, by id or a prefix of
		// it, the latest when empty.
		Snapshot string

		// Into is a directory to restore aside into.
		Into string

		// Config asks the configuration of the object restored: in
		// place, it alone, aside, beside the heads.
		Config bool

		// ByRID asks each head restored into the current head of the
		// resource of its rid, rather than at the path it was saved
		// from.
		ByRID bool

		// Stop asks the instance brought to the state a restore in place
		// needs, and left stopped.
		Stop bool

		// Heads are the heads to restore, every head of the snapshot
		// when empty.
		Heads []string

		// Refs is how the referenced keys are restored: missing, skip
		// or overwrite.
		Refs string
	}

	// OptTo sets a barrier when iterating over a resource lister
	OptTo struct {
		To     string
		UpTo   string // Deprecated
		DownTo string // Deprecated
	}
)
