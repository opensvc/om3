package rescontainerocibase

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnterEnv(t *testing.T) {
	configured := &InspectData{InspectDataConfig: InspectDataConfig{Env: []string{"PATH=/usr/local/bin:/usr/bin:/bin", "A=1"}}}
	env, err := enterEnv(configured, os.Getpid())
	require.NoError(t, err)
	require.Equal(t, []string{"PATH=/usr/local/bin:/usr/bin:/bin", "A=1"}, env, "the configured environment, not the process one")

	t.Setenv("OM3_ENTER_ENV_TEST", "x")
	env, err = enterEnv(&InspectData{}, os.Getpid())
	require.NoError(t, err)
	require.NotEmpty(t, env, "no configured environment: the one of the process")
}
