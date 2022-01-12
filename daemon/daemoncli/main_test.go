package daemoncli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDaemonStartThenStop(t *testing.T) {
	require.Nil(t, Start())
	require.Nil(t, Stop())
}
