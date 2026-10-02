package sizeconv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A share is a percentage, with or without its sign, or a size, binary
// whatever the unit is written as.
func TestParseShare(t *testing.T) {
	for _, tc := range []struct {
		s    string
		want Share
	}{
		{"10", Share{Percent: 10}},
		{"10%", Share{Percent: 10}},
		{" 2 % ", Share{Percent: 2}},
		{"0", Share{Percent: 0}},
		{"100%", Share{Percent: 100}},
		{"512m", Share{Size: 512 << 20, IsSize: true}},
		{"2g", Share{Size: 2 << 30, IsSize: true}},
		{"2Gi", Share{Size: 2 << 30, IsSize: true}},
		{"2GiB", Share{Size: 2 << 30, IsSize: true}},
	} {
		got, err := ParseShare(tc.s)
		require.NoError(t, err, tc.s)
		assert.Equal(t, tc.want, got, tc.s)
	}
	for _, s := range []string{"", "101%", "-1", "1.5%", "ten", "2Q"} {
		_, err := ParseShare(s)
		assert.Error(t, err, s)
	}
}

// A share resolves to a percentage of a whole. A size beyond the limit is
// read as the limit, and a share of an unknown whole is none.
func TestSharePercentOf(t *testing.T) {
	const gib = int64(1) << 30
	total := 16 * gib
	for _, tc := range []struct {
		name  string
		share Share
		total int64
		limit int
		want  int
	}{
		{"a percentage", Share{Percent: 10}, total, 50, 10},
		{"a percentage over the limit is the user's", Share{Percent: 80}, total, 50, 80},
		{"a size", Share{Size: 2 * gib, IsSize: true}, total, 50, 12},
		{"a size truncates", Share{Size: 100 << 20, IsSize: true}, total, 50, 0},
		{"a size over the limit", Share{Size: 12 * gib, IsSize: true}, total, 50, 50},
		{"a size over the whole", Share{Size: 32 * gib, IsSize: true}, total, 100, 100},
		{"an unknown whole", Share{Percent: 10}, 0, 50, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.share.PercentOf(tc.total, tc.limit))
		})
	}
}
