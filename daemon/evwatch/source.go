package evwatch

import (
	"context"
	"errors"
	"time"
)

type (
	// Source is the event source of a watcher, as a tool printing the
	// events it reads.
	Source interface {
		// Name names the source in the logs, as "drbdsetup events2".
		Name() string

		// Check says whether the source can run on the node. An error
		// wrapping ErrUnsupported says it can not, as a tool not
		// installed or too old: the source is not tried again before the
		// next resource of its kind. Another error is retried.
		Check(ctx context.Context) error

		// Run reads the events until the context is done or the source
		// ends. It calls ready once the events are read, which a resync
		// follows, and emit for each event, with the key of the resource
		// it is about. It returns once nothing runs any more.
		Run(ctx context.Context, ready func(), emit func(key string)) error
	}

	// Policy says how a Runner runs its source.
	Policy struct {
		// StopGrace is how long the source runs on once no resource of
		// its kind is left, so a provision retried or an object deleted
		// and created again does not stop and start it.
		StopGrace time.Duration

		// RestartMin is the delay before the source runs again once it
		// ended, doubled at each quick failure up to RestartMax.
		RestartMin time.Duration
		RestartMax time.Duration

		// QuickFailure is the run a source ending sooner than fails
		// quickly. A longer run resets the count of quick failures.
		QuickFailure time.Duration

		// MaxQuickFailures is how many quick failures in a row disable
		// the source until no resource of its kind is left. Zero never
		// disables it.
		MaxQuickFailures int

		// CheckTimeout bounds a Check.
		CheckTimeout time.Duration
	}
)

// ErrUnsupported is the error of a source that can not run on the node.
var ErrUnsupported = errors.New("unsupported")

// DefaultPolicy is the policy of a source never disabled: a service it
// connects to can be down for long and come back.
var DefaultPolicy = Policy{
	StopGrace:    60 * time.Second,
	RestartMin:   5 * time.Second,
	RestartMax:   5 * time.Minute,
	QuickFailure: 10 * time.Second,
	CheckTimeout: 10 * time.Second,
}
