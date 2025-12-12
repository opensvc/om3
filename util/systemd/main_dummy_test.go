//go:build !linux

package systemd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHasSystemd(t *testing.T) {
	t.Run("returns false on non systemd systems", func(t *testing.T) {
		require.False(t, HasSystemd())
	})
}
