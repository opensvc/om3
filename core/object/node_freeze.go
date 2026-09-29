package object

import (
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/core/freeze"
)

// lockName is the path of the file to use as an action lock.
func (t *Node) frozenFile() string {
	return filepath.Join(t.VarDir(), "frozen")
}

// Frozen returns the unix timestamp of the last freeze.
func (t *Node) Frozen() time.Time {
	return freeze.Frozen(t.frozenFile())
}

// FrozenScope returns the scope of the freeze of the node, empty when it
// is not frozen.
func (t *Node) FrozenScope() freeze.Scope {
	return freeze.ScopeOf(t.frozenFile(), freeze.ScopeCluster, freeze.ScopeNode)
}

// Freeze creates a persistent flag file that prevents orchestration
// of the object instance.
//
// The flag records a freeze of the cluster when the node monitor says so,
// for the freeze to reach the nodes that missed it.
func (t *Node) Freeze() error {
	freezeFn := freeze.Freeze
	if os.Getenv(env.FreezeScopeVar) == string(freeze.ScopeCluster) {
		freezeFn = func(p string) error {
			return freeze.FreezeScope(p, freeze.ScopeCluster)
		}
	}
	if err := freezeFn(t.frozenFile()); err != nil {
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
