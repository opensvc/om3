package actioncontext

import (
	"context"
	"testing"

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
