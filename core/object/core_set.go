package object

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/keyop"
	"opensvc.com/opensvc/core/xconfig"
)

// Set changes or adds a keyword and its value in the configuration file.
func (t *core) Set(ctx context.Context, kops ...keyop.T) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.Set)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return setKeys(t.config, kops...)
}

func (t *core) setKeys(kops ...keyop.T) error {
	return setKeys(t.config, kops...)
}

func setKeywords(cf *xconfig.T, kws []string) error {
	l := keyop.ParseOps(kws)
	return setKeys(cf, l...)
}

func setKeys(cf *xconfig.T, kops ...keyop.T) error {
	changes := 0
	for _, op := range kops {
		if op.IsZero() {
			return fmt.Errorf("invalid set expression: %s", op)
		}
		log.Debug().
			Stringer("key", op.Key).
			Stringer("op", op.Op).
			Str("val", op.Value).
			Msg("set")
		if err := cf.Set(op); err != nil {
			return err
		}
		changes++
	}
	if changes > 0 {
		return cf.Commit()
	}
	return nil
}
