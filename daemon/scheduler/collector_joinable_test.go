package scheduler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
)

// The jobs requiring the collector are held while the speaker finds it
// unreachable, on the speaker and on the nodes following it, and run again
// once it answers.
func TestCollectorJoinable(t *testing.T) {
	since := time.Now()
	url := "http://oc3:8080"
	collector := func(state, speaker string, unreachable bool) daemonsubsystem.Collector {
		c := daemonsubsystem.Collector{Url: url, Speaker: speaker}
		c.State = state
		if unreachable {
			c.UnreachableSince = &since
		}
		return c
	}
	cases := []struct {
		name       string
		collectors map[string]daemonsubsystem.Collector
		joinable   bool
	}{
		{"speaker answered", map[string]daemonsubsystem.Collector{"n1": collector("speaker", "n1", false)}, true},
		{"speaker unreachable", map[string]daemonsubsystem.Collector{"n1": collector("speaker", "n1", true)}, false},
		{"following an unreachable speaker", map[string]daemonsubsystem.Collector{
			"n1": collector("speaker-candidate", "n2", false),
			"n2": collector("speaker", "n2", true),
		}, false},
		{"following a speaker answered", map[string]daemonsubsystem.Collector{
			"n1": collector("speaker-candidate", "n2", true),
			"n2": collector("speaker", "n2", false),
		}, true},
		{"a former speaker unreachable", map[string]daemonsubsystem.Collector{
			"n1": collector("speaker-candidate", "n2", false),
			"n2": collector("speaker", "n2", false),
			"n3": collector("speaker-candidate", "n2", true),
		}, true},
		{"following a speaker not known yet", map[string]daemonsubsystem.Collector{"n1": collector("speaker-candidate", "n2", false)}, false},
		{"no client", map[string]daemonsubsystem.Collector{"n1": collector("warning", "n1", false)}, false},
		{"local unknown", map[string]daemonsubsystem.Collector{"n2": collector("speaker", "n2", false)}, false},
	}
	for _, tc := range cases {
		joinable, reason := collectorJoinable("n1", tc.collectors)
		assert.Equalf(t, tc.joinable, joinable, "%s: %s", tc.name, reason)
	}
}
