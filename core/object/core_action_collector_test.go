package object

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/xsession"
)

// TestBeginCollectorActionReportsOnce pins that an action chained by
// another one, as the start of a restart, does not report on its own.
func TestBeginCollectorActionReportsOnce(t *testing.T) {
	rec := &collectorAction{}
	ctx := context.WithValue(context.Background(), collectorActionKey{}, rec)

	var a actor
	got, done := a.beginCollectorAction(ctx, "start")
	assert.Equal(t, ctx, got, "the record of the chaining action is kept")
	done(nil, nil)
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

// TestBeginCollectorActionChainedPanic pins that the end of an action
// chained by another one passes a panic it recovered on to the chaining
// action.
func TestBeginCollectorActionChainedPanic(t *testing.T) {
	ctx := context.WithValue(context.Background(), collectorActionKey{}, &collectorAction{})
	var a actor
	_, done := a.beginCollectorAction(ctx, "start")
	assert.PanicsWithValue(t, "boom", func() { done(nil, "boom") })
}

// TestBeginCollectorActionPanicEndsErr pins that an action panicking is
// reported failed, not succeeded as its error is still nil, and that the
// panic goes on.
func TestBeginCollectorActionPanicEndsErr(t *testing.T) {
	testhelper.Setup(t)
	t.Cleanup(func() { rawconfig.Load(map[string]string{}) })
	_, err := SetClusterConfig()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(rawconfig.NodeConfigFile(), []byte(
		"[node]\nuuid = 00000000-0000-0000-0000-000000000001\n[collector]\nfeeder = https://collector.invalid/feeder\n"), 0600))

	p, _ := naming.ParsePath("test/svc/svc1")
	o, err := NewSvc(p, WithConfigData([]byte("[fs#1]\ntype = flag\n")))
	require.NoError(t, err)

	run := func() (err error) {
		_, done := o.beginCollectorAction(context.Background(), "start")
		defer func() { done(err, recover()) }()
		panic("boom")
	}
	assert.PanicsWithValue(t, "boom", func() { _ = run() }, "the panic goes on")

	dir := collector.ActionPendingDir(rawconfig.CollectorActionPendingDir())
	a, err := dir.Read(collector.ActionKey(xsession.ExecID().UUID(), p), collector.ActionPhaseEnd)
	require.NoError(t, err, "the end is recorded")
	assert.Equal(t, "err", a.Status)
}
