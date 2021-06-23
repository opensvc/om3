package actioncontext

import (
	"context"
	"time"

	"opensvc.com/opensvc/core/actionrollback"
	"opensvc.com/opensvc/core/objectactionprops"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/resourceselector"
)

type (
	key int
	T   struct {
		Options interface{}
		Props   objectactionprops.T
	}
	isConfirmer interface {
		IsConfirm() bool
	}
	isForcer interface {
		IsForce() bool
	}
	isLeaderer interface {
		IsLeader() bool
	}
	toStrer interface {
		ToStr() string
	}
	isDryRuner interface {
		IsDryRun() bool
	}
	isRollbackDisableder interface {
		IsRollbackDisabled() bool
	}
	resourceSelectorOptionser interface {
		ResourceSelectorOptions() resourceselector.Options
	}
	ResourceLister interface {
		Resources() resource.Drivers
		IsDesc() bool
	}
)

const (
	tKey key = 0
)

func New(options interface{}, props objectactionprops.T) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	ctx = context.WithValue(ctx, tKey, &T{
		Props:   props,
		Options: options,
	})
	if props.Rollback {
		ctx = actionrollback.NewContext(ctx)
	}
	return ctx, cancel
}

func value(ctx context.Context) *T {
	return ctx.Value(tKey).(*T)
}

func Props(ctx context.Context) objectactionprops.T {
	return value(ctx).Props
}

func NewResourceSelector(ctx context.Context, l ResourceLister) *resourceselector.T {
	opts := ResourceSelectorOptions(ctx)
	order := Props(ctx).Order
	return resourceselector.New(
		l,
		resourceselector.WithOptions(opts),
		resourceselector.WithOrder(order),
	)
}

func ResourceSelectorOptions(ctx context.Context) resourceselector.Options {
	if o, ok := value(ctx).Options.(resourceSelectorOptionser); ok {
		return o.ResourceSelectorOptions()
	}
	return resourceselector.Options{}
}

func To(ctx context.Context) string {
	if o, ok := value(ctx).Options.(toStrer); ok {
		return o.ToStr()
	}
	return ""
}

func IsConfirm(ctx context.Context) bool {
	if o, ok := value(ctx).Options.(isConfirmer); ok {
		return o.IsConfirm()
	}
	return false
}

func IsDryRun(ctx context.Context) bool {
	if o, ok := value(ctx).Options.(isDryRuner); ok {
		return o.IsDryRun()
	}
	return false
}

func IsRollbackDisabled(ctx context.Context) bool {
	if o, ok := value(ctx).Options.(isRollbackDisableder); ok {
		return o.IsRollbackDisabled()
	}
	return false
}

func IsForce(ctx context.Context) bool {
	if o, ok := value(ctx).Options.(isForcer); ok {
		return o.IsForce()
	}
	return false
}

func IsLeader(ctx context.Context) bool {
	if o, ok := value(ctx).Options.(isLeaderer); ok {
		return o.IsLeader()
	}
	return false
}
