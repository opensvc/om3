package node

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// The load pool interval of a config holding none, as the config of a node of
// a version before the keyword, is the default, and a lower one than the
// minimum is raised to it: the api never reports 0.
func TestEffectiveLoadPoolInterval(t *testing.T) {
	for set, want := range map[time.Duration]time.Duration{
		0:                      DefaultLoadPoolInterval,
		-time.Second:           DefaultLoadPoolInterval,
		500 * time.Millisecond: MinLoadPoolInterval,
		time.Second:            time.Second,
		5 * time.Minute:        5 * time.Minute,
	} {
		assert.Equal(t, want, Config{LoadPoolInterval: set}.EffectiveLoadPoolInterval(), "%s", set)
	}
}
