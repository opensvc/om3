package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/resourceselector"
	"github.com/opensvc/om3/v3/core/statusbus"
	"github.com/opensvc/om3/v3/util/key"
)

// Disable unsets disable=true from the svc config
func (t *svc) Disable(ctx context.Context) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.Disable)
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	defer t.postActionStatusEval(ctx)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	var kops keyop.L
	rs := resourceselector.FromContext(ctx, t)
	if rs.IsZero() {
		kops = append(kops, keyop.T{
			Key: key.T{
				Section: "DEFAULT",
				Option:  "disable",
			},
			Op:    keyop.Set,
			Value: "true",
		})
	} else {
		for _, r := range rs.Resources() {
			kops = append(kops, keyop.T{
				Key: key.T{
					Section: r.RID(),
					Option:  "disable",
				},
				Op:    keyop.Set,
				Value: "true",
			})
		}
	}
	if err := t.checkDisable(kops); err != nil {
		return err
	}
	return t.config.Set(kops...)
}

// checkDisable refuses to disable a resource up on a node, as any change a
// resource up would leak by: it would be left running with nothing to stop
// it, and what it holds would be held by a configuration saying nothing of
// it. The daemon knows the statuses of every node, a command run without it
// the one of this node.
func (t *svc) checkDisable(kops keyop.L) error {
	to, err := NewConfigurer(t.path, WithVolatile(true))
	if err != nil {
		return err
	}
	if err := to.Config().PrepareSet(kops...); err != nil {
		return err
	}
	statuses := instance.StatusData.GetByPath(t.path)
	if len(statuses) == 0 {
		if statuses, err = localInstanceStatuses(t.path); err != nil {
			return err
		}
	}
	return CheckStopToChange(t.path.Kind, t.config, to.Config(), statuses)
}
