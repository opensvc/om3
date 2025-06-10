package rescontainerpodman

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/drivers/rescontainerocibase"
)

func Test_ExecBaseArgs(t *testing.T) {
	d := &T{CNIConfig: "/test-cni-config.d"}

	if err := d.Configure(); err != nil {
		require.NoError(t, err)
	}

	baseArgs := d.
		Executer().(rescontainerocibase.ExecutorArgserGetter).
		ExecutorArgser().(rescontainerocibase.ExecutorBaseArgser).
		ExecBaseArgs()

	expectedBaseArgs := []string{"--cni-config-dir", "/test-cni-config.d"}
	require.ElementsMatchf(t, expectedBaseArgs, baseArgs, "want: %s\ngot:  %s", expectedBaseArgs, baseArgs)
}
