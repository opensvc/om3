package actioncontext

import (
	"context"
	"slices"
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
	envKey
	forceKey
	leaderKey
	lockTimeoutKey
	lockDisabledKey
	masterKey
	propsKey
	quietKey
	ridKey
	rollbackDisabledKey
	slaveKey
	slavesKey
	subsetKey
	tagKey
	targetKey
	toKey
	verboseKey
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

func WithQuiet(ctx context.Context, v bool) context.Context {
	return context.WithValue(ctx, quietKey, v)
}
func IsQuiet(ctx context.Context) bool {
	if i := ctx.Value(quietKey); i != nil {
		return i.(bool)
	}
	return false
}

func WithVerbose(ctx context.Context, v int) context.Context {
	return context.WithValue(ctx, verboseKey, v)
}
func Verbose(ctx context.Context) int {
	if i := ctx.Value(verboseKey); i != nil {
		return i.(int)
	}
	return 0
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

func WithMaster(ctx context.Context, v bool) context.Context {
	return context.WithValue(ctx, masterKey, v)
}
func IsMaster(ctx context.Context) bool {
	if i := ctx.Value(masterKey); i != nil {
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

func WithEnv(ctx context.Context, s []string) context.Context {
	return context.WithValue(ctx, envKey, s)
}
func Env(ctx context.Context) []string {
	if i := ctx.Value(envKey); i != nil {
		return i.([]string)
	}
	return []string{}
}

func WithSlaves(ctx context.Context, s []string) context.Context {
	return context.WithValue(ctx, slaveKey, s)
}
func Slaves(ctx context.Context) []string {
	if i := ctx.Value(slaveKey); i != nil {
		return i.([]string)
	}
	return []string{}
}

func WithAllSlaves(ctx context.Context, v bool) context.Context {
	return context.WithValue(ctx, slavesKey, v)
}
func IsAllSlaves(ctx context.Context) bool {
	if i := ctx.Value(slavesKey); i != nil {
		return i.(bool)
	}
	return false
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

func IsActionForSlave(ctx context.Context, nodename string) bool {
	if IsAllSlaves(ctx) {
		return true
	}
	slaves := Slaves(ctx)
	if slices.Contains(slaves, nodename) {
		return true
	}
	if !IsMaster(ctx) && len(slaves) == 0 {
		return true
	}
	return false
}

func IsActionForMaster(ctx context.Context) bool {
	if IsMaster(ctx) {
		return true
	}
	if !IsAllSlaves(ctx) && len(Slaves(ctx)) == 0 {
		return true
	}
	return false
}
