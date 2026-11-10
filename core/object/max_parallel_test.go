package object

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// node.max_parallel takes 0, the number of cpus, or the minimum at least: a
// lower one the daemon does not honor, and would raise.
func TestValidMaxParallel(t *testing.T) {
	for _, s := range []string{"0", "2", "10", " 4 "} {
		assert.NoError(t, validMaxParallel(s), s)
	}
	for _, s := range []string{"1", "-1", "x"} {
		assert.Error(t, validMaxParallel(s), s)
	}
}

// node.load_pool_interval takes 1s at least: a lower one, 0 included, would
// have the daemon load the pools without pause.
func TestValidLoadPoolInterval(t *testing.T) {
	for _, s := range []string{"1s", "5m", " 90s ", "60", "", " "} {
		assert.NoError(t, validLoadPoolInterval(s), s)
	}
	for _, s := range []string{"0", "0s", "500ms", "-1s", "x"} {
		assert.Error(t, validLoadPoolInterval(s), s)
	}
}

// collector.speaker_grace_period takes 1s at least: a lower one, 0 included,
// would have the nodes take the speech of a speaker restarting at once.
func TestValidSpeakerGracePeriod(t *testing.T) {
	for _, s := range []string{"1s", "1m", " 45s ", "60", "", " "} {
		assert.NoError(t, validSpeakerGracePeriod(s), s)
	}
	for _, s := range []string{"0", "0s", "500ms", "-1s", "x"} {
		assert.Error(t, validSpeakerGracePeriod(s), s)
	}
}
