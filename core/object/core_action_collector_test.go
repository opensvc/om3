package object

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/testhelper"
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

func TestMaskArgvEnv(t *testing.T) {
	argv := []string{"/usr/bin/om", "s1", "run", "--rid", "task#1", "--env", "TOKEN=secret", "--env=PASS=other", "--env", "NOEQUAL"}
	assert.Equal(t,
		[]string{"/usr/bin/om", "s1", "run", "--rid", "task#1", "--env", "TOKEN=xxx", "--env=PASS=xxx", "--env", "xxx"},
		maskArgv(argv),
		"an --env value keeps the name of the variable it sets")
	assert.Equal(t,
		[]string{"om", "--value", "xxx", "--rid", "x"},
		maskArgv([]string{"om", "--value", "--env", "--rid", "x"}),
		"the argument following --value is its value, as the flag parser reads it, whatever it looks like")
}

// TestCollectorActionRIDsWithoutProps pins that an action chaining others,
// as a restart, reports the resources selected by --rid, --tag or
// --subset, though its context carries no action properties.
func TestCollectorActionRIDsWithoutProps(t *testing.T) {
	testhelper.Setup(t)
	_, err := SetClusterConfig()
	require.NoError(t, err)
	cf := []byte(`
[fs#1]
type = flag
tags = db

[fs#2]
type = flag
subset = s2

[fs#3]
type = flag
tags = db
subset = s2
`)
	p, _ := naming.ParsePath("test/svc/svc1")
	o, err := NewSvc(p, WithConfigData(cf))
	require.NoError(t, err)

	for name, tc := range map[string]struct {
		ctx  context.Context
		want []string
	}{
		"no selection": {context.Background(), nil},
		"rid":          {actioncontext.WithRID(context.Background(), "fs#2"), []string{"fs#2"}},
		"tag":          {actioncontext.WithTag(context.Background(), "db"), []string{"fs#1", "fs#3"}},
		"subset":       {actioncontext.WithSubset(context.Background(), "s2"), []string{"fs#2", "fs#3"}},
	} {
		t.Run(name, func(t *testing.T) {
			got := o.collectorActionRIDs(tc.ctx)
			if tc.want == nil {
				assert.Empty(t, got)
				return
			}
			assert.ElementsMatch(t, tc.want, strings.Split(got, ","))
		})
	}
}
