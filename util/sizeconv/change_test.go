package sizeconv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseChange(t *testing.T) {
	const g = 1024 * 1024 * 1024
	for _, tc := range []struct {
		s        string
		value    int64
		relative bool
		resolved int64 // against a current size of 10g
	}{
		{"11g", 11 * g, false, 11 * g},
		{"12Gi", 12 * g, false, 12 * g},
		{"11GB", 11000000000, false, 11000000000},
		{"+1g", g, true, 11 * g},
		{"-1g", -g, true, 9 * g},
		{" +1g ", g, true, 11 * g},
	} {
		change, err := ParseChange(tc.s)
		require.NoErrorf(t, err, "ParseChange(%q)", tc.s)
		assert.Equalf(t, tc.value, change.Value, "ParseChange(%q).Value", tc.s)
		assert.Equalf(t, tc.relative, change.IsRelative, "ParseChange(%q).IsRelative", tc.s)
		assert.Equalf(t, tc.resolved, change.Resolve(10*g), "ParseChange(%q).Resolve(10g)", tc.s)
	}
}

func TestParseChangeRejects(t *testing.T) {
	for _, s := range []string{"", " ", "+", "-", "1x", "gg", "+-1g"} {
		_, err := ParseChange(s)
		assert.Errorf(t, err, "ParseChange(%q) must be refused", s)
	}
}
