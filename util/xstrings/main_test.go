package xstrings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSwapCase(t *testing.T) {
	t.Run("TrimLast(Foo, 1) => Fo", func(t *testing.T) {
		assert.Equal(t, TrimLast("Foo", 1), "Fo")
	})
	t.Run("TrimLast(Foo, 2) => Fo", func(t *testing.T) {
		assert.Equal(t, TrimLast("Foo", 2), "F")
	})
	t.Run("SwapRuneCase(F) => f", func(t *testing.T) {
		assert.Equal(t, SwapRuneCase('F'), 'f')
	})
	t.Run("SwapCase(Foo) => fOO", func(t *testing.T) {
		assert.Equal(t, SwapCase("Foo"), "fOO")
	})
}
