package duration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFmtShortDuration(t *testing.T) {
	cases := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"zero", 0, "0s"},
		{"seconds", 45*time.Second + 30*time.Millisecond, "45s"},
		{"minutes and seconds", 6*time.Minute + 43*time.Second, "6m43s"},
		{"minutes", 31*time.Minute + 12*time.Second, "31m"},
		{"hours and minutes", 2*time.Hour + 15*time.Minute + 5*time.Second, "2h15m"},
		{"hours", 5 * time.Hour, "5h"},
		{"hours 2", 17*time.Hour + 41*time.Minute + 32*time.Second, "17h"},
		{"day and hours", 3*24*time.Hour + 7*time.Hour + 30*time.Minute, "3d7h"},
		{"days", 10 * 24 * time.Hour, "10d"},
		{"days 2", 25*24*time.Hour + 5*time.Hour, "25d"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FmtShortDuration(tc.d)
			assert.Equal(t, tc.want, got)
		})
	}
}

// A duration shorter than a second matches none of the units the largest-unit
// loop walks, which left it on the first: 300ms rendered "0d".
func TestFmtShortDurationUnderASecond(t *testing.T) {
	for _, tc := range []struct {
		d        time.Duration
		expected string
	}{
		{0, "0s"},
		{-time.Second, "0s"},
		{300 * time.Millisecond, "300ms"},
		{999 * time.Millisecond, "999ms"},
		{812 * time.Microsecond, "812us"},
		{time.Second, "1s"},
		{90 * time.Second, "1m30s"},
		{25 * time.Hour, "1d1h"},
	} {
		assert.Equalf(t, tc.expected, FmtShortDuration(tc.d), "%s", tc.d)
	}
}
