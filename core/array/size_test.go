package array

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSize(t *testing.T) {
	const gib = int64(1024 * 1024 * 1024)
	cases := map[string]Size{
		"1073741824": {Bytes: gib},
		"10g":        {Bytes: 10 * gib},
		"10G":        {Bytes: 10 * gib},
		"10gb":       {Bytes: 10 * gib},
		"10GB":       {Bytes: 10 * gib},
		"10GiB":      {Bytes: 10 * gib},
		"1.5g":       {Bytes: gib + gib/2},
		"1,5g":       {Bytes: gib + gib/2},
		"1,5 GB":     {Bytes: gib + gib/2},
		"512m":       {Bytes: 512 * 1024 * 1024},
		"512MB":      {Bytes: 512 * 1024 * 1024},
		"1t":         {Bytes: 1024 * gib},
		"+1g":        {Bytes: gib, Relative: true},
		"+512MiB":    {Bytes: 512 * 1024 * 1024, Relative: true},
		" 2g ":       {Bytes: 2 * gib},
	}
	for s, want := range cases {
		t.Run(s, func(t *testing.T) {
			got, err := ParseSize(s)
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
	for _, s := range []string{"", "-1g", "g", "10x", "10gz", "abc", "0", "1..2g", "1,,2g", "1,2.3g", "8e", "9e"} {
		t.Run("refuse "+s, func(t *testing.T) {
			_, err := ParseSize(s)
			assert.Error(t, err)
		})
	}
}

func TestSizeTargetAndCheckResize(t *testing.T) {
	grow, err := ParseSize("+1g")
	require.NoError(t, err)
	assert.Equal(t, int64(3<<30), grow.Target(2<<30))

	abs, err := ParseSize("1g")
	require.NoError(t, err)
	assert.Equal(t, int64(1<<30), abs.Target(2<<30))

	assert.NoError(t, CheckResize(1<<30, 2<<30, false), "a growth")
	assert.NoError(t, CheckResize(1<<30, 1<<30, false), "the same size")
	err = CheckResize(2<<30, 1<<30, false)
	require.Error(t, err, "a shrink")
	assert.True(t, errors.Is(err, ErrShrink))
	assert.NoError(t, CheckResize(2<<30, 1<<30, true), "a shrink allowed by --truncate")
}
