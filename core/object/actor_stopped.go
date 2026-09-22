package object

import (
	"time"

	"github.com/opensvc/om3/v3/core/flagfile"
)

// StoppedAt returns when the instance was stopped on purpose, or the zero
// time when it was not.
func (t *actor) StoppedAt() time.Time {
	return flagfile.At(t.path.StoppedFile())
}

// SetStopped says the instance was stopped on purpose, so the daemon does not
// start it back on its own. A stop raises this flag where it used to freeze
// the instance, which left the operator unable to tell their own freeze from
// the ones an orchestration made.
func (t *actor) SetStopped() error {
	return flagfile.Set(t.path.StoppedFile())
}

// UnsetStopped says the instance is wanted up again, so the daemon may start
// it on its own. Every start the user asks for clears the flag, on the
// instances that stay down as well, so a failover still has candidates.
func (t *actor) UnsetStopped() error {
	return flagfile.Unset(t.path.StoppedFile())
}
