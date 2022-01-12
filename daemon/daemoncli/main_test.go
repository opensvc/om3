package daemoncli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDaemonStartThenStop(t *testing.T) {
	require.False(t, Running())
	require.Nil(t, Start())
	require.True(t, Running())
	require.Nil(t, Stop())
	require.False(t, Running())
}
