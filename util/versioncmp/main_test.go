package versioncmp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseKeepsTheNumericComponents(t *testing.T) {
	for s, want := range map[string][]int{
		"4.0.12":                   {4, 0, 12},
		"v8.0.0":                   {8, 0, 0},
		"5.0.0~git2209-g5a7b9ce67": {5, 0, 0},
		"4.0.12-0ubuntu1~20.04.1":  {4, 0, 12},
		"0.8.8\n":                  {0, 8, 8},
		"2.1.":                     {2, 1},
	} {
		got, err := Parse(s)
		require.NoError(t, err, s)
		assert.Equal(t, want, got, s)
	}
	for _, s := range []string{"", "git", "v", "~1.0"} {
		_, err := Parse(s)
		assert.Error(t, err, s)
	}
}

func TestCompare(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"2.1", "2.1.0", 0},
		{"2.1.1", "2.1", 1},
		{"2.0.9", "2.1", -1},
		{"0.10.0", "0.7.8", 1},
		{"5.0.0~git2209", "2.1", 1},
	} {
		got, err := Compare(c.a, c.b)
		require.NoError(t, err)
		assert.Equal(t, c.want, got, "%s vs %s", c.a, c.b)
	}
}

func TestThresholds(t *testing.T) {
	ok, err := AtLeast("0.7.8", "0.7.8")
	require.NoError(t, err)
	assert.True(t, ok, "a version is at least itself")

	ok, err = Newer("2.1.0", "2.1")
	require.NoError(t, err)
	assert.False(t, ok, "2.1.0 is not newer than 2.1")

	_, err = Newer("unknown", "2.1")
	assert.Error(t, err)
}
