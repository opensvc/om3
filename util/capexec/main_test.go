package capexec

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parsed(t *testing.T, args ...string) T {
	t.Helper()
	var x T
	flags := pflag.NewFlagSet("exec", pflag.ContinueOnError)
	x.FlagSet(flags)
	require.NoError(t, flags.Parse(args))
	return x
}

// A limit whose value is not one is refused, naming its flag, rather than
// ignored: a limit ignored is one the user believes applied.
func TestToLimitRefusesAnInvalidValueNamingItsFlag(t *testing.T) {
	_, err := parsed(t, "--limit-nofile", "lots", "--limit-cpu", "soon").toLimit()
	require.Error(t, err)
	assert.ErrorContains(t, err, `--limit-nofile "lots"`)
	assert.ErrorContains(t, err, `--limit-cpu "soon"`)

	limit, err := parsed(t, "--limit-nofile", "64").toLimit()
	require.NoError(t, err)
	require.NotNil(t, limit.NoFile)
	assert.Equal(t, int64(64), *limit.NoFile)
}

// A process group cap with no process group is refused: it would be ignored.
func TestAPGCapNeedsThePG(t *testing.T) {
	err := parsed(t, "--pg-mem-limit", "64m", "--pg-pids-max", "10").checkPG()
	require.Error(t, err)
	assert.ErrorContains(t, err, "--pg-mem-limit --pg-pids-max")
	assert.NoError(t, parsed(t, "--pg", "/opensvc.slice/x.slice", "--pg-mem-limit", "64m").checkPG())
	assert.NoError(t, parsed(t).checkPG())
}

// The caps an app passes to om exec are the ones om exec parses, the ones
// added with the unified hierarchy included.
func TestArgvOfTheCapsIsParsedBack(t *testing.T) {
	x := parsed(t, "--pg", "/a.slice", "--pg-mem-high", "300m", "--pg-pids-max", "7", "--pg-cpu-burst", "25%@4")
	y := parsed(t, x.Argv()...)
	c := y.toPG()
	assert.Equal(t, "/a.slice", c.ID)
	assert.Equal(t, "300m", c.MemHigh)
	assert.Equal(t, "7", c.PidsMax)
	assert.Equal(t, "25%@4", c.CPUBurst)
}
