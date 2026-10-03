package object

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/testhelper"
)

// TestCollectorRawConfig pins that the collector settings are read from the
// [collector] section, and from the deprecated node.collector* keywords
// where the section does not set them.
func TestCollectorRawConfig(t *testing.T) {
	for name, tc := range map[string]struct {
		cluster string
		node    string
		check   func(t *testing.T, cfg *collector.Config)
	}{
		"the defaults": {
			check: func(t *testing.T, cfg *collector.Config) {
				assert.Empty(t, cfg.FeederUrl)
				assert.Equal(t, 60*time.Second, cfg.PingInterval)
				assert.Equal(t, 10*time.Second, cfg.StatusDelay)
				assert.Equal(t, 5*time.Second, cfg.Timeout)
				assert.Equal(t, 100, cfg.ActionBatch)
				assert.Equal(t, 10*time.Second, cfg.ActionLogTimeout)
			},
		},
		"the collector section": {
			cluster: "[collector]\nurl = https://oc3\nping_interval = 2m\nstatus_delay = 20s\ntimeout = 8s\naction_batch = 20\naction_log_timeout = 30s\n",
			check: func(t *testing.T, cfg *collector.Config) {
				assert.Equal(t, "https://oc3/feeder", cfg.FeederUrl, "derived from the url")
				assert.Equal(t, "https://oc3/server", cfg.ServerUrl)
				assert.Equal(t, 2*time.Minute, cfg.PingInterval)
				assert.Equal(t, 20*time.Second, cfg.StatusDelay)
				assert.Equal(t, 8*time.Second, cfg.Timeout)
				assert.Equal(t, 20, cfg.ActionBatch)
				assert.Equal(t, 30*time.Second, cfg.ActionLogTimeout)
			},
		},
		"the deprecated node keywords": {
			cluster: "[node]\ncollector = https://old\ncollector_server = https://old-server\ncollector_ping_interval = 3m\ncollector_timeout = 7s\n",
			check: func(t *testing.T, cfg *collector.Config) {
				assert.Equal(t, "https://old/feeder", cfg.FeederUrl)
				assert.Equal(t, "https://old-server", cfg.ServerUrl)
				assert.Equal(t, 3*time.Minute, cfg.PingInterval)
				assert.Equal(t, 7*time.Second, cfg.Timeout)
			},
		},
		"an om2 alias": {
			cluster: "[node]\ndb_min_ping_interval = 4m\n",
			check: func(t *testing.T, cfg *collector.Config) {
				assert.Equal(t, 4*time.Minute, cfg.PingInterval)
			},
		},
		"the collector section over the deprecated keywords": {
			cluster: "[collector]\nfeeder = https://new/feeder\nstatus_delay = 15s\n",
			node:    "[node]\ncollector_feeder = https://old/feeder\ncollector_status_delay = 30s\ncollector_timeout = 9s\n",
			check: func(t *testing.T, cfg *collector.Config) {
				assert.Equal(t, "https://new/feeder", cfg.FeederUrl)
				assert.Equal(t, 15*time.Second, cfg.StatusDelay)
				assert.Equal(t, 9*time.Second, cfg.Timeout, "a setting the section does not set falls back")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			testhelper.Setup(t)
			t.Cleanup(func() { rawconfig.Load(map[string]string{}) })
			require.NoError(t, os.WriteFile(rawconfig.ClusterConfigFile(), []byte("[cluster]\nname = c1\n"+tc.cluster), 0600))
			require.NoError(t, os.WriteFile(rawconfig.NodeConfigFile(), []byte(tc.node), 0600))
			n, err := NewNode()
			require.NoError(t, err)
			tc.check(t, n.CollectorRawConfig().AsConfig())
		})
	}
}
