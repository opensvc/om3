package xstrings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrimLast(t *testing.T) {
	t.Run("TrimLast(F, 2)=>", func(t *testing.T) {
		assert.Equal(t, TrimLast("F", 2), "")
	})
	t.Run("TrimLast(F, 1)=>", func(t *testing.T) {
		assert.Equal(t, TrimLast("F", 1), "")
	})
	t.Run("TrimLast(Foo, 1)=>Fo", func(t *testing.T) {
		assert.Equal(t, TrimLast("Foo", 1), "Fo")
	})
	t.Run("TrimLast(Foo, 2)=>Fo", func(t *testing.T) {
		assert.Equal(t, TrimLast("Foo", 2), "F")
	})
}

func TestSwapCase(t *testing.T) {
	t.Run("SwapRuneCase(F)=>f", func(t *testing.T) {
		assert.Equal(t, SwapRuneCase('F'), 'f')
	})
	t.Run("SwapCase()=>", func(t *testing.T) {
		assert.Equal(t, SwapCase(""), "")
	})
	t.Run("SwapCase(F)=>f", func(t *testing.T) {
		assert.Equal(t, SwapCase("F"), "f")
	})
	t.Run("SwapCase(Foo)=>fOO", func(t *testing.T) {
		assert.Equal(t, SwapCase("Foo"), "fOO")
	})
}

func TestCapitalize(t *testing.T) {
	t.Run("Capitalize()=>", func(t *testing.T) {
		assert.Equal(t, Capitalize(""), "")
	})
	t.Run("Capitalize(f)=>F", func(t *testing.T) {
		assert.Equal(t, Capitalize("f"), "F")
	})
	t.Run("Capitalize(F)=>F", func(t *testing.T) {
		assert.Equal(t, Capitalize("F"), "F")
	})
	t.Run("Capitalize(foo foo)=>Foo foo", func(t *testing.T) {
		assert.Equal(t, Capitalize("foo foo"), "Foo foo")
	})
	t.Run("Capitalize(FOO foo)=>FOO foo", func(t *testing.T) {
		assert.Equal(t, Capitalize("FOO foo"), "FOO foo")
	})
}
