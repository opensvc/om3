package actioncontext

import (
	"context"
	"time"

	"github.com/opensvc/om3/core/actionrollback"
	"github.com/opensvc/om3/util/pg"
)

type (
	key int
)

const (
	confirmKey key = iota
	cronKey
	propsKey
	ridKey
	tagKey
	subsetKey
	forceKey
	lockTimeoutKey
	lockDisabledKey
	toKey
	dryRunKey
	rollbackDisabledKey
	leaderKey
	targetKey
)

func WithLockDisabled(ctx context.Context, v bool) context.Context {
	return context.WithValue(ctx, lockDisabledKey, v)
}
func IsLockDisabled(ctx context.Context) bool {
	if i := ctx.Value(lockDisabledKey); i != nil {
		return i.(bool)
	}
	return false
}

func WithRollbackDisabled(ctx context.Context, v bool) context.Context {
	return context.WithValue(ctx, rollbackDisabledKey, v)
}
func IsRollbackDisabled(ctx context.Context) bool {
	if i := ctx.Value(rollbackDisabledKey); i != nil {
		return i.(bool)
	}
	return false
}

func WithDryRun(ctx context.Context, v bool) context.Context {
	return context.WithValue(ctx, dryRunKey, v)
}
func IsDryRun(ctx context.Context) bool {
	if i := ctx.Value(dryRunKey); i != nil {
		return i.(bool)
	}
	return false
}

func WithLeader(ctx context.Context, v bool) context.Context {
	return context.WithValue(ctx, leaderKey, v)
}
func IsLeader(ctx context.Context) bool {
	if i := ctx.Value(leaderKey); i != nil {
		return i.(bool)
	}
	return false
}

func WithConfirm(ctx context.Context, v bool) context.Context {
	return context.WithValue(ctx, confirmKey, v)
}
func IsConfirm(ctx context.Context) bool {
	if i := ctx.Value(confirmKey); i != nil {
		return i.(bool)
	}
	return false
}

func WithCron(ctx context.Context, v bool) context.Context {
	return context.WithValue(ctx, cronKey, v)
}
func IsCron(ctx context.Context) bool {
	if i := ctx.Value(cronKey); i != nil {
		return i.(bool)
	}
	return false
}

func WithForce(ctx context.Context, v bool) context.Context {
	return context.WithValue(ctx, forceKey, v)
}
func IsForce(ctx context.Context) bool {
	if i := ctx.Value(forceKey); i != nil {
		return i.(bool)
	}
	return false
}

func WithTo(ctx context.Context, s string) context.Context {
	return context.WithValue(ctx, toKey, s)
}
func To(ctx context.Context) string {
	if i := ctx.Value(toKey); i != nil {
		return i.(string)
	}
	return ""
}

func WithRID(ctx context.Context, s string) context.Context {
	return context.WithValue(ctx, ridKey, s)
}
func RID(ctx context.Context) string {
	if i := ctx.Value(ridKey); i != nil {
		return i.(string)
	}
	return ""
}

func WithTag(ctx context.Context, s string) context.Context {
	return context.WithValue(ctx, tagKey, s)
}
func Tag(ctx context.Context) string {
	if i := ctx.Value(tagKey); i != nil {
		return i.(string)
	}
	return ""
}

func WithSubset(ctx context.Context, s string) context.Context {
	return context.WithValue(ctx, subsetKey, s)
}
func Subset(ctx context.Context) string {
	if i := ctx.Value(subsetKey); i != nil {
		return i.(string)
	}
	return ""
}

func WithTarget(ctx context.Context, s []string) context.Context {
	return context.WithValue(ctx, targetKey, s)
}
func Target(ctx context.Context) []string {
	if i := ctx.Value(targetKey); i != nil {
		return i.([]string)
	}
	return []string{}
}

func WithLockTimeout(ctx context.Context, d time.Duration) context.Context {
	return context.WithValue(ctx, lockTimeoutKey, d)
}
func LockTimeout(ctx context.Context) time.Duration {
	if i := ctx.Value(lockTimeoutKey); i != nil {
		return i.(time.Duration)
	}
	return 5 * time.Second
}

func WithProps(ctx context.Context, props Properties) context.Context {
	ctx = context.WithValue(ctx, propsKey, props)
	if props.Rollback {
		ctx = actionrollback.NewContext(ctx)
	}
	if props.PG {
		ctx = pg.NewContext(ctx)
	}
	return ctx
}

func Props(ctx context.Context) Properties {
	return ctx.Value(propsKey).(Properties)
}
