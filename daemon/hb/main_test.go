package hb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestIsStaleMsg pins which peer messages are dropped as older than the last
// one processed: a slow heartbeat delivering a superseded message must not set
// back the peer gens, and a peer whose clock was stepped back must not be
// ignored for long.
func TestIsStaleMsg(t *testing.T) {
	last := time.Date(2026, 10, 4, 10, 28, 55, 0, time.UTC)
	for name, tc := range map[string]struct {
		updated time.Time
		last    time.Time
		want    bool
	}{
		"first message of the peer":                  {updated: last, last: time.Time{}, want: false},
		"newer message":                              {updated: last.Add(time.Second), last: last, want: false},
		"message a slow heartbeat read late":         {updated: last.Add(-10 * time.Second), last: last, want: true},
		"message as old as a heartbeat timeout":      {updated: last.Add(-time.Minute), last: last, want: true},
		"message of a peer whose clock was set back": {updated: last.Add(-time.Hour), last: last, want: false},
		"message just past the stale max age":        {updated: last.Add(-staleMsgMaxAge), last: last, want: false},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, isStaleMsg(tc.updated, tc.last))
		})
	}
}
