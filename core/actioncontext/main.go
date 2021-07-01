package actioncontext

import (
	"context"

	"opensvc.com/opensvc/core/actionrollback"
	"opensvc.com/opensvc/core/objectactionprops"
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
)

const (
	tKey key = 0
)

func New(options interface{}, props objectactionprops.T) context.Context {
	ctx := context.WithValue(context.Background(), tKey, &T{
		Props:   props,
		Options: options,
	})
	if props.Rollback {
		ctx = actionrollback.NewContext(ctx)
	}
	return ctx
}

func Value(ctx context.Context) *T {
	return ctx.Value(tKey).(*T)
}

func Options(ctx context.Context) interface{} {
	return Value(ctx).Options
}

func Props(ctx context.Context) objectactionprops.T {
	return Value(ctx).Props
}

func To(ctx context.Context) string {
	if o, ok := Value(ctx).Options.(toStrer); ok {
		return o.ToStr()
	}
	return ""
}

func IsConfirm(ctx context.Context) bool {
	if o, ok := Value(ctx).Options.(isConfirmer); ok {
		return o.IsConfirm()
	}
	return false
}

func IsDryRun(ctx context.Context) bool {
	if o, ok := Value(ctx).Options.(isDryRuner); ok {
		return o.IsDryRun()
	}
	return false
}

func IsRollbackDisabled(ctx context.Context) bool {
	if o, ok := Value(ctx).Options.(isRollbackDisableder); ok {
		return o.IsRollbackDisabled()
	}
	return false
}

func IsForce(ctx context.Context) bool {
	if o, ok := Value(ctx).Options.(isForcer); ok {
		return o.IsForce()
	}
	return false
}

func IsLeader(ctx context.Context) bool {
	if o, ok := Value(ctx).Options.(isLeaderer); ok {
		return o.IsLeader()
	}
	return false
}
