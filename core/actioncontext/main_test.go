package actioncontext

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsResourceSelected(t *testing.T) {
	t.Run("no selection recorded", func(t *testing.T) {
		selected, known := IsResourceSelected(context.Background(), "fs#1")
		assert.False(t, selected, "selected")
		assert.False(t, known, "known")
	})

	ctx := WithSelectedRIDs(context.Background(), []string{"app#1", "fs#1"})

	t.Run("selected", func(t *testing.T) {
		selected, known := IsResourceSelected(ctx, "fs#1")
		assert.True(t, selected, "selected")
		assert.True(t, known, "known")
	})

	t.Run("not selected", func(t *testing.T) {
		selected, known := IsResourceSelected(ctx, "ip#1")
		assert.False(t, selected, "selected")
		assert.True(t, known, "known")
	})

	t.Run("empty selection", func(t *testing.T) {
		selected, known := IsResourceSelected(WithSelectedRIDs(context.Background(), nil), "fs#1")
		assert.False(t, selected, "selected")
		assert.True(t, known, "known")
	})
}
