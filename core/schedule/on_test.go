package schedule

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An event is matched by its kind, and the values of the fields the filter
// says.
func TestOnFilterMatch(t *testing.T) {
	f, err := ParseOn("NetworkPeersUpdated")
	require.NoError(t, err)
	assert.True(t, f.Match("NetworkPeersUpdated,node=n1,name=backend1"))
	assert.False(t, f.Match("NetLinkUp,node=n1,link_name=eth0"))

	f, err = ParseOn("NetworkPeersUpdated,name=backend1")
	require.NoError(t, err)
	assert.True(t, f.Match("NetworkPeersUpdated,node=n1,name=backend1"))
	assert.False(t, f.Match("NetworkPeersUpdated,node=n1,name=backend2"))
	assert.False(t, f.Match("NetworkPeersUpdated,node=n1,name=backend10"))

	_, err = ParseOn("NodeMonitorUpdated")
	assert.Error(t, err, "not an event an action runs on")
	_, err = ParseOn("NetworkPeersUpdated,name")
	assert.Error(t, err, "a field without value")
}

func TestParsePace(t *testing.T) {
	for _, tc := range []struct {
		s        string
		interval time.Duration
		max      time.Duration
		wantErr  bool
	}{
		{s: "", interval: 10 * time.Second, max: 10 * time.Second},
		{s: "@30s", interval: 30 * time.Second, max: 30 * time.Second},
		{s: "@2", interval: 2 * time.Minute, max: 2 * time.Minute},
		{s: "@30s backoff @1m", interval: 30 * time.Second, max: time.Minute},
		{s: "30s", wantErr: true},
		{s: "@0", wantErr: true},
		{s: "@1m backoff @30s", wantErr: true},
		{s: "@30s @1m", wantErr: true},
		{s: "@30s backoff", wantErr: true},
	} {
		t.Run(tc.s, func(t *testing.T) {
			pace, err := ParsePace(tc.s)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.interval, pace.Interval)
			assert.Equal(t, tc.max, pace.Max)
		})
	}
}

// A copy of a table keeps every field of its entries, the events and the
// pace of a task included, and shares no slice with it.
func TestTableDeepCopyKeepsEveryField(t *testing.T) {
	table := Table{{Config: Config{Action: "run", Key: "task#1.schedule", On: []string{"NetworkPeersUpdated"}, Pace: "@30s"}}}
	c := table.DeepCopy()
	assert.Equal(t, table, *c)
	(*c)[0].On[0] = "FileInstalled"
	assert.Equal(t, "NetworkPeersUpdated", table[0].On[0])
}
