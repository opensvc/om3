package systemd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasSystemd(t *testing.T) {
	t.Run("returns true on systemd systems", func(t *testing.T) {
		assert.True(t, HasSystemd())
	})
}

func TestJoinAgentCgroup(t *testing.T) {
	t.Run("puts the pid in the opensvc-agent cgroup", func(t *testing.T) {
		JoinAgentCgroup()
	})
}
