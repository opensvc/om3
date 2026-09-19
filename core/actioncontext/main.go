package actioncontext

import (
	"context"
	"slices"
	"time"

	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/pg"
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
	moveToKey
	propsKey
	quietKey
	ridKey
	rollbackDisabledKey
	selectedRIDsKey
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
func Master(ctx context.Context) bool {
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

func WithMoveTo(ctx context.Context, v string) context.Context {
	return context.WithValue(ctx, moveToKey, v)
}
func MoveTo(ctx context.Context) string {
	if i := ctx.Value(moveToKey); i != nil {
		return i.(string)
	}
	return ""
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

// selection is the value WithSelectedRIDs stores: the rids are only
// meaningful for the resources of the object at path.
type selection struct {
	path naming.Path
	rids map[string]struct{}
}

// WithSelectedRIDs records the rids of the resources an action with a
// resource selection works on or reads the state of, for the object at path.
// Drivers read it with IsResourceSelected, for example to spare a provider
// api call on a resource the action does not depend on.
//
// The path scopes the rids: an action can evaluate the status of another
// object with the same context, like the start affinity checks do, and the
// resources of that object must not be judged by this selection.
func WithSelectedRIDs(ctx context.Context, path naming.Path, rids []string) context.Context {
	m := make(map[string]struct{}, len(rids))
	for _, rid := range rids {
		m[rid] = struct{}{}
	}
	return context.WithValue(ctx, selectedRIDsKey, selection{path: path, rids: m})
}

// IsResourceSelected tells whether the resource identified by rid, of the
// object at path, is among the ones recorded by WithSelectedRIDs. The known
// return value is false when no selection is recorded for this object, i.e.
// outside an action, during an action on all the resources, or during the
// status evaluation of another object, so the caller can apply its default
// policy.
func IsResourceSelected(ctx context.Context, path naming.Path, rid string) (selected, known bool) {
	v, ok := ctx.Value(selectedRIDsKey).(selection)
	if !ok || v.path != path {
		return false, false
	}
	_, selected = v.rids[rid]
	return selected, true
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
func AllSlaves(ctx context.Context) bool {
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
	if props.PG && pg.FromContext(ctx) == nil {
		ctx = pg.NewContext(ctx)
	}
	return ctx
}

func Props(ctx context.Context) Properties {
	return ctx.Value(propsKey).(Properties)
}

func IsActionForSlave(ctx context.Context, nodename string) bool {
	if AllSlaves(ctx) {
		return true
	}
	slaves := Slaves(ctx)
	if slices.Contains(slaves, nodename) {
		return true
	}
	if !Master(ctx) && len(slaves) == 0 {
		return true
	}
	return false
}

func IsActionForMaster(ctx context.Context) bool {
	if Master(ctx) {
		return true
	}
	if !AllSlaves(ctx) && len(Slaves(ctx)) == 0 {
		return true
	}
	return false
}

func HasResourceSelector(ctx context.Context) bool {
	return RID(ctx) != "" || Tag(ctx) != "" || Subset(ctx) != "" || To(ctx) != ""
}
