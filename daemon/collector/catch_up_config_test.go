package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/core/node"
)

// A collector configuration changed between the read at start and the
// subscription to its changes, as a collector url unset right after the
// daemon started, is applied once the collector is subscribed. One that did
// not change is not applied again.
func TestCatchUpConfig(t *testing.T) {
	node.InitData()
	c, _ := newTestT(t)
	c.nodeIsLeader = make(map[string]bool)
	c.appliedConfig = &collector.Config{FeederUrl: "https://collector.example.com"}
	c.status.Url = "unknown"

	node.ConfigData.Set("node1", &node.Config{})
	c.catchUpConfig()
	assert.Nil(t, c.appliedConfig, "the collector url unset since the start is applied")
	assert.True(t, c.disable)
	assert.Empty(t, c.status.Url)

	configuredAt := c.status.ConfiguredAt
	c.catchUpConfig()
	assert.Equal(t, configuredAt, c.status.ConfiguredAt, "the configuration applied is not applied again")
}
