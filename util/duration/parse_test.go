package duration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseReadsWhatTheAgentPrints pins that a duration the agent shows can be
// written back into a configuration.
func TestParseReadsWhatTheAgentPrints(t *testing.T) {
	cases := []struct {
		s        string
		expected time.Duration
	}{
		{"1h", time.Hour},
		{"30m", 30 * time.Minute},
		{"1d", 24 * time.Hour},
		{"10d", 240 * time.Hour},
		{"3d7h", 79 * time.Hour},
		{"3d7h30m", 79*time.Hour + 30*time.Minute},
		{"0.5d", 12 * time.Hour},
		{"-1d", -24 * time.Hour},
		{"1h30m", 90 * time.Minute},
		{"90s", 90 * time.Second},
	}
	for _, tc := range cases {
		got, err := Parse(tc.s)
		require.NoErrorf(t, err, "Parse(%q)", tc.s)
		assert.Equalf(t, tc.expected, got, "Parse(%q)", tc.s)
	}
}

// TestParseRefusesWhatItCannotRead keeps Parse as strict as the standard
// library about everything but the day.
func TestParseRefusesWhatItCannotRead(t *testing.T) {
	for _, s := range []string{"", "1", "d", "1x", "one hour", "1dd"} {
		_, err := Parse(s)
		assert.Errorf(t, err, "Parse(%q) must fail", s)
	}
}

// TestParseReadsBackFmtShortDuration is the round trip that matters: whatever
// the agent prints, it can read.
func TestParseReadsBackFmtShortDuration(t *testing.T) {
	for _, d := range []time.Duration{
		time.Hour,
		24 * time.Hour,
		240 * time.Hour,
		79 * time.Hour,
		90 * time.Second,
	} {
		s := FmtShortDuration(d)
		got, err := Parse(s)
		require.NoErrorf(t, err, "Parse(FmtShortDuration(%s)) = %q", d, s)
		assert.Equalf(t, d, got, "round trip of %s through %q", d, s)
	}
}

// TestDurationUnmarshalsTheDayUnit covers the configuration path, which is
// where an access_token_duration of "1d" is read.
func TestDurationUnmarshalsTheDayUnit(t *testing.T) {
	var d Duration
	require.NoError(t, d.UnmarshalJSON([]byte(`"1d"`)))
	assert.Equal(t, 24*time.Hour, d.Duration)

	require.NoError(t, d.UnmarshalJSON([]byte(`"1h"`)))
	assert.Equal(t, time.Hour, d.Duration)

	assert.Error(t, d.UnmarshalJSON([]byte(`"1x"`)))
}

// TestFlagAcceptsTheDayUnit covers the command line path.
func TestFlagAcceptsTheDayUnit(t *testing.T) {
	var d time.Duration
	f := NewFlag(&d)
	require.NoError(t, f.Set("1d"))
	assert.Equal(t, 24*time.Hour, d)
	assert.Equal(t, "duration", f.Type())
	assert.Error(t, f.Set("1x"))
}
