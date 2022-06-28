package actioncontext

import (
	"context"
	"time"

	"opensvc.com/opensvc/core/actionrollback"
	"opensvc.com/opensvc/core/objectactionprops"
	"opensvc.com/opensvc/util/pg"
)

type (
	key      int
	isCroner interface {
		IsCron() bool
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
	isLockDisableder interface {
		IsLockDisabled() bool
	}
	lockTimeouter interface {
		LockTimeout() time.Duration
	}
	rider interface {
		ResourceSelectorRID() string
	}
	tager interface {
		ResourceSelectorTag() string
	}
	subseter interface {
		ResourceSelectorSubset() string
	}
)

const (
	optionsKey key = iota
	propsKey
)

func WithOptions(ctx context.Context, options interface{}) context.Context {
	return context.WithValue(ctx, optionsKey, options)
}

func Options(ctx context.Context) interface{} {
	return ctx.Value(optionsKey)
}

func WithProps(ctx context.Context, props objectactionprops.T) context.Context {
	ctx = context.WithValue(ctx, propsKey, props)
	if props.Rollback {
		ctx = actionrollback.NewContext(ctx)
	}
	if props.PG {
		ctx = pg.NewContext(ctx)
	}
	return ctx
}

func Props(ctx context.Context) objectactionprops.T {
	return ctx.Value(propsKey).(objectactionprops.T)
}

func To(ctx context.Context) string {
	if o, ok := Options(ctx).(toStrer); ok {
		return o.ToStr()
	}
	return ""
}

func IsConfirm(ctx context.Context) bool {
	if o, ok := Options(ctx).(isConfirmer); ok {
		return o.IsConfirm()
	}
	return false
}

func IsCron(ctx context.Context) bool {
	if o, ok := Options(ctx).(isCroner); ok {
		return o.IsCron()
	}
	return false
}

func IsDryRun(ctx context.Context) bool {
	if o, ok := Options(ctx).(isDryRuner); ok {
		return o.IsDryRun()
	}
	return false
}

func IsRollbackDisabled(ctx context.Context) bool {
	if o, ok := Options(ctx).(isRollbackDisableder); ok {
		return o.IsRollbackDisabled()
	}
	return false
}

func IsForce(ctx context.Context) bool {
	if o, ok := Options(ctx).(isForcer); ok {
		return o.IsForce()
	}
	return false
}

func IsLeader(ctx context.Context) bool {
	if o, ok := Options(ctx).(isLeaderer); ok {
		return o.IsLeader()
	}
	return false
}

func IsLockDisabled(ctx context.Context) bool {
	if o, ok := Options(ctx).(isLockDisableder); ok {
		return o.IsLockDisabled()
	}
	return false
}

func LockTimeout(ctx context.Context) time.Duration {
	if o, ok := Options(ctx).(lockTimeouter); ok {
		return o.LockTimeout()
	}
	return time.Second * 0
}

func ResourceSelectorRID(ctx context.Context) string {
	if o, ok := Options(ctx).(rider); ok {
		return o.ResourceSelectorRID()
	}
	return ""
}

func ResourceSelectorTag(ctx context.Context) string {
	if o, ok := Options(ctx).(tager); ok {
		return o.ResourceSelectorTag()
	}
	return ""
}

func ResourceSelectorSubset(ctx context.Context) string {
	if o, ok := Options(ctx).(subseter); ok {
		return o.ResourceSelectorSubset()
	}
	return ""
}
