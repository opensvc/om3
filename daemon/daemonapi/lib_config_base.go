package daemonapi

import (
	"errors"
	"fmt"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/xconfig"
)

// ErrConfigChanged is a configuration write refused because the file changed
// after the write was checked against it.
var ErrConfigChanged = xconfig.ErrConfigChanged

// configBase is the configuration file a write was checked against.
//
// A write is checked, by the rbac policy, the validation and the claims,
// against the configuration read before it, and lands as a whole file. A
// file changed in between, as another user may change it, is replaced by
// the one checked against the older file: a keyword the other write changed
// is put back to the value it had, without the grant that change needs being
// asked of anyone. A cap holds its write until its orchestration is queued,
// which makes the window wide enough to hit. So the write lands only if the
// file is still the one it was checked against, and is refused otherwise,
// for the caller to ask again against the file as it now is.
//
// It is read before anything the checks read, so a file changed while they
// read it is one changed since the base, and is refused too. The comparison
// and the write are made by xconfig, under the lock of the file every writer
// takes.
type configBase struct {
	p    naming.Path
	base xconfig.Base
}

func readConfigBase(p naming.Path) (configBase, error) {
	base, err := xconfig.ReadBase(p.ConfigFile())
	if err != nil {
		return configBase{}, err
	}
	return configBase{p: p, base: base}, nil
}

// commit runs the write of cfg, landing only over the base.
func (t configBase) commit(cfg *xconfig.T, write func() error) error {
	cfg.SetBase(t.base)
	err := write()
	if errors.Is(err, ErrConfigChanged) {
		return fmt.Errorf("%w: the configuration of %s was written by someone else meanwhile, ask again", ErrConfigChanged, t.p)
	}
	return err
}
