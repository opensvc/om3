package object

import (
	"context"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/xconfig"
)

// Set changes or adds a keyword and its value in the configuration file.
func (t *core) Set(ctx context.Context, kops ...keyop.T) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.Set)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.config.SetKeys(kops...)
}

func (t *core) setKeys(kops ...keyop.T) error {
	return t.config.SetKeys(kops...)
}

func setKeywords(cf *xconfig.T, kws []string) error {
	l := keyop.ParseOps(kws)
	return cf.SetKeys(l...)
}
