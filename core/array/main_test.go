package array

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test(t *testing.T) {
	t.Run("valid expression 1", func(t *testing.T) {
		m, err := ParseMappings([]string{"a:b"})
		require.Nil(t, err)
		require.Len(t, m, 1)
		require.Contains(t, m, "a:b")
	})
	t.Run("valid expression 2", func(t *testing.T) {
		m, err := ParseMappings([]string{"a:b,c"})
		require.Nil(t, err)
		require.Len(t, m, 2)
		require.Contains(t, m, "a:b")
		require.Contains(t, m, "a:c")
	})
	t.Run("invalid expression 1", func(t *testing.T) {
		_, err := ParseMappings([]string{"a:b,"})
		require.NotNil(t, err)
	})
	t.Run("invalid expression 2", func(t *testing.T) {
		_, err := ParseMappings([]string{"a:"})
		require.NotNil(t, err)
	})
	t.Run("invalid expression 3", func(t *testing.T) {
		_, err := ParseMappings([]string{"a"})
		require.NotNil(t, err)
	})
}
