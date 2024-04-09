package object

import (
	"context"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/resourceselector"
	"github.com/opensvc/om3/core/statusbus"
	"github.com/opensvc/om3/util/key"
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
	var kws key.L
	rs := resourceselector.FromContext(ctx, t)
	if rs.IsZero() {
		kws = append(kws, key.T{"DEFAULT", "disable"})
	} else {
		for _, r := range rs.Resources() {
			kws = append(kws, key.T{r.RID(), "disable"})
		}
	}
	return t.config.Unset(kws...)
}
