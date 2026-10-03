package object

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskArgv(t *testing.T) {
	argv := []string{"s1", "set", "--kw", "env.pass", "--value", "secret", "--value=other"}
	masked := maskArgv(argv)
	assert.Equal(t, []string{"s1", "set", "--kw", "env.pass", "--value", "xxx", "--value=xxx"}, masked)
	assert.Equal(t, "secret", argv[5], "the argv given is left untouched")
	assert.Equal(t, []string{"--value"}, maskArgv([]string{"--value"}), "a trailing --value has nothing to mask")
}

// TestBeginCollectorActionReportsOnce pins that an action chained by
// another one, as the start of a restart, does not report on its own.
func TestBeginCollectorActionReportsOnce(t *testing.T) {
	rec := &collectorAction{}
	ctx := context.WithValue(context.Background(), collectorActionKey{}, rec)

	var a actor
	got, done := a.beginCollectorAction(ctx, "start")
	assert.Equal(t, ctx, got, "the record of the chaining action is kept")
	done(nil)
	assert.True(t, rec.a.End.IsZero(), "the chained action does not end the report")

	// an action not to report marks its context, so the actions it chains
	// do not report either
	notReported := context.WithValue(context.Background(), collectorActionKey{}, (*collectorAction)(nil))
	got, _ = a.beginCollectorAction(notReported, "start")
	assert.Equal(t, notReported, got)
}

func TestWithCollectorActionOf(t *testing.T) {
	rec := &collectorAction{}
	from := context.WithValue(context.Background(), collectorActionKey{}, rec)
	ctx := withCollectorActionOf(context.Background(), from)
	assert.Same(t, rec, ctx.Value(collectorActionKey{}))

	ctx = withCollectorActionOf(context.Background(), context.Background())
	assert.Nil(t, ctx.Value(collectorActionKey{}))
}
