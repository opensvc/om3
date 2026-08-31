package daemonapi

import (
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSignal(t *testing.T) {
	t.Run("accepts names, prefixed names and numbers", func(t *testing.T) {
		for _, s := range []string{"term", "TERM", "sigterm", "SIGTERM", " SIGTERM ", "15"} {
			sig, err := parseSignal(s)
			require.NoError(t, err, "parseSignal(%q)", s)
			assert.Equal(t, syscall.SIGTERM, sig, "parseSignal(%q)", s)
		}
	})
	t.Run("refuses unknown signals", func(t *testing.T) {
		for _, s := range []string{"", "foo", "SIGFOO", "0", "-1", "1234"} {
			_, err := parseSignal(s)
			assert.Error(t, err, "parseSignal(%q)", s)
		}
	})
}
