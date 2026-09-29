package commoncmd

import (
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

func TestFlagsLockWaitLock(t *testing.T) {
	parse := func(args ...string) OptsLock {
		var o OptsLock
		flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
		FlagsLock(flags, &o)
		require.NoError(t, flags.Parse(args))
		return o
	}
	o := parse()
	require.Equal(t, 30*time.Second, o.Timeout, "the default")
	require.False(t, o.TimeoutSet)

	o = parse("--waitlock", "5s")
	require.Equal(t, 5*time.Second, o.Timeout)
	require.True(t, o.TimeoutSet, "given by the user")

	o = parse("--waitlock", "30s")
	require.True(t, o.TimeoutSet, "given, even with the default value")
}
