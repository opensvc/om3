package resource

import (
	"context"
	"errors"
	"fmt"

	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/statusbus"
)

// QuiesceAll suspends the writes of the resources of l that are up and
// quiesce something, in the order of l, as the order of a stop, and returns
// how to resume them, in the reverse order. A quiesce failing resumes the
// resources already quiesced, and is returned.
func QuiesceAll(ctx context.Context, l []Driver) (func(context.Context) error, error) {
	sb := statusbus.FromContext(ctx)
	resumes := make([]func(context.Context) error, 0)
	resume := func(ctx context.Context) error {
		var errs error
		for i := len(resumes) - 1; i >= 0; i-- {
			errs = errors.Join(errs, resumes[i](ctx))
		}
		return errs
	}
	for _, r := range l {
		q, ok := r.(Quiescer)
		if !ok || r.IsDisabled() || !isUp(sb, r.RID()) {
			continue
		}
		rid := r.RID()
		f, err := q.Quiesce(ctx)
		if err != nil {
			err = fmt.Errorf("%s: %w", rid, err)
			if rerr := resume(ctx); rerr != nil {
				err = errors.Join(err, rerr)
			}
			return nil, err
		}
		if f != nil {
			resumes = append(resumes, func(ctx context.Context) error {
				if err := f(ctx); err != nil {
					return fmt.Errorf("%s: %w", rid, err)
				}
				return nil
			})
		}
	}
	return resume, nil
}

// isUp says the status bus knows the resource rid up, as a standby one.
func isUp(sb *statusbus.ObjT, rid string) bool {
	if sb == nil {
		return false
	}
	return sb.Get(rid).Is(status.Up, status.StandbyUp)
}
