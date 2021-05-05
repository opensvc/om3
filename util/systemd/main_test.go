// +build linux

package systemd

import (
	"testing"

	require "github.com/stretchr/testify/require"
)

func TestHasSystemd(t *testing.T) {
	t.Run("returns true on systemd systems", func(t *testing.T) {
		require.True(t, HasSystemd())
	})
}
