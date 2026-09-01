package daemonapi

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
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
