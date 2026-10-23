package actioncontext

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/naming"
)

func TestIsResourceSelected(t *testing.T) {
	var (
		path      = naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "svc1"}
		otherPath = naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "svc2"}
	)

	t.Run("no selection recorded", func(t *testing.T) {
		selected, known := IsResourceSelected(context.Background(), path, "fs#1")
		assert.False(t, selected, "selected")
		assert.False(t, known, "known")
	})

	ctx := WithSelectedRIDs(context.Background(), path, []string{"app#1", "fs#1"})

	t.Run("selected", func(t *testing.T) {
		selected, known := IsResourceSelected(ctx, path, "fs#1")
		assert.True(t, selected, "selected")
		assert.True(t, known, "known")
	})

	t.Run("not selected", func(t *testing.T) {
		selected, known := IsResourceSelected(ctx, path, "ip#1")
		assert.False(t, selected, "selected")
		assert.True(t, known, "known")
	})

	t.Run("empty selection", func(t *testing.T) {
		selected, known := IsResourceSelected(WithSelectedRIDs(context.Background(), path, nil), path, "fs#1")
		assert.False(t, selected, "selected")
		assert.True(t, known, "known")
	})

	t.Run("another object", func(t *testing.T) {
		for _, rid := range []string{"fs#1", "ip#1"} {
			selected, known := IsResourceSelected(ctx, otherPath, rid)
			assert.False(t, selected, "%s selected", rid)
			assert.False(t, known, "%s known", rid)
		}
	})
}

func TestWithoutResourceSelector(t *testing.T) {
	ctx := context.Background()
	ctx = WithRID(ctx, "volume#1")
	ctx = WithTag(ctx, "t1")
	ctx = WithSubset(ctx, "g1")
	ctx = WithTo(ctx, "volume#1")
	ctx = WithLeader(ctx, true)
	ctx = WithForce(ctx, true)
	assert.True(t, HasResourceSelector(ctx), "selector before")

	ctx = WithoutResourceSelector(ctx)
	assert.False(t, HasResourceSelector(ctx), "selector after")
	assert.True(t, IsLeader(ctx), "leader kept")
	assert.True(t, IsForce(ctx), "force kept")
}

// --force-affinity lifts the hard affinity checks only, and --force lifts
// them with every other check.
func TestIsForceAffinity(t *testing.T) {
	ctx := context.Background()
	assert.False(t, IsForceAffinity(ctx))
	assert.True(t, IsForceAffinity(WithForceAffinity(ctx, true)))
	assert.False(t, IsForce(WithForceAffinity(ctx, true)), "force affinity is not force")
	assert.True(t, IsForceAffinity(WithForce(ctx, true)), "force implies force affinity")
	assert.True(t, IsForceAffinity(WithForceAffinity(WithForce(ctx, true), false)))
}

func TestBeganAt(t *testing.T) {
	// The resources of one action, acting at different times of it, read
	// the one time it began.
	began := time.Date(2026, 10, 23, 23, 59, 58, 0, time.UTC)
	ctx := WithBeganAt(context.Background(), began)
	assert.Equal(t, began, BeganAt(ctx))
	assert.Equal(t, began, BeganAt(WithRID(ctx, "sync#2")))

	// Outside an action, it is now.
	before := time.Now()
	assert.False(t, BeganAt(context.Background()).Before(before))
}
