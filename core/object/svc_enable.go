package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/resourceselector"
	"github.com/opensvc/om3/v3/core/statusbus"
	"github.com/opensvc/om3/v3/util/key"
)

// Enable unsets disable=true from the svc config
func (t *svc) Enable(ctx context.Context) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.Enable)
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	defer t.postActionStatusEval(ctx)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.config.Unset(t.enableKeys(ctx)...)
}

// enableKeys returns the disable keywords an enable unsets: the one of the
// object, or the ones of the resources selected.
func (t *svc) enableKeys(ctx context.Context) key.L {
	var kws key.L
	rs := resourceselector.FromContext(ctx, t)
	if rs.IsZero() {
		kws = append(kws, key.T{
			Section: "DEFAULT",
			Option:  "disable",
		})
	} else {
		for _, r := range rs.Resources() {
			kws = append(kws, key.T{
				Section: r.RID(),
				Option:  "disable",
			})
		}
	}
	return kws
}

// EnabledConfig returns the configuration the object would have once
// enabled, for the checks of a configuration written to judge it before
// the enable writes it: a resource enabled holds its values again.
func (t *svc) EnabledConfig(ctx context.Context) ([]byte, error) {
	ctx = actioncontext.WithProps(ctx, actioncontext.Enable)
	to, err := NewConfigurer(t.path, WithVolatile(true))
	if err != nil {
		return nil, err
	}
	if err := to.Config().PrepareUnset(t.enableKeys(ctx)...); err != nil {
		return nil, err
	}
	return to.Config().Dump()
}
