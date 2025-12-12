package object

import (
	"path/filepath"
	"time"

	"github.com/opensvc/om3/v3/core/freeze"
	"github.com/rs/zerolog/log"
)

// lockName is the path of the file to use as an action lock.
func (t *Node) frozenFile() string {
	return filepath.Join(t.VarDir(), "frozen")
}

// Frozen returns the unix timestamp of the last freeze.
func (t *Node) Frozen() time.Time {
	return freeze.Frozen(t.frozenFile())
}

// Freeze creates a persistent flag file that prevents orchestration
// of the object instance.
func (t *Node) Freeze() error {
	if err := freeze.Freeze(t.frozenFile()); err != nil {
		return err
	}
	log.Info().Msg("now frozen")
	return nil
}

// Unfreeze removes the persistent flag file that prevents orchestration
// of the object instance.
func (t *Node) Unfreeze() error {
	if err := freeze.Unfreeze(t.frozenFile()); err != nil {
		return err
	}
	log.Info().Msg("now unfrozen")
	return nil
}
