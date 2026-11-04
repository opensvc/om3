package commoncmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/daemon/api"
)

// The age of a relay client data is rendered from its update time, and a
// relay not answering, with no update time, has none.
func TestRelayViewsRenderTheAge(t *testing.T) {
	now := time.Now()
	l := ToRelayViews(api.RelayStatusItems{
		{Nodename: "live", UpdatedAt: now.Add(-3 * time.Second)},
		{Nodename: "unreachable", Status: "dial tcp: refused"},
	}, now)
	assert.Equal(t, "3s", l[0].Age)
	assert.Equal(t, "", l[1].Age)
}
