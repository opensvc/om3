package daemonapi

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

// TestLogControlLevel pins the spelling the api answers with, which is
// the spelling its body takes: zerolog renders the level that emits
// nothing as an empty string, and an empty level in the body means "none
// was asked for".
func TestLogControlLevel(t *testing.T) {
	previous := zerolog.GlobalLevel()
	defer zerolog.SetGlobalLevel(previous)

	for _, tc := range []struct {
		level zerolog.Level
		want  string
	}{
		{zerolog.InfoLevel, "info"},
		{zerolog.WarnLevel, "warn"},
		{zerolog.ErrorLevel, "error"},
		{zerolog.NoLevel, "none"},
		{zerolog.Disabled, "none"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			zerolog.SetGlobalLevel(tc.level)
			assert.Equal(t, tc.want, logControlLevel())
		})
	}
}

// TestRetryAfterSeconds pins the pacing hint a refused caller gets: the
// header is in seconds, and a limiter granting more than one token per
// second still has to say something a client can wait for.
func TestRetryAfterSeconds(t *testing.T) {
	for _, tc := range []struct {
		rate rate.Limit
		want int
	}{
		{20, 1},
		{1, 1},
		{0.5, 2},
		{0.1, 10},
		{0, 1},
		{-1, 1},
	} {
		assert.Equal(t, tc.want, retryAfterSeconds(tc.rate), "rate %v", tc.rate)
	}
}
