package object

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
)

// The listener throttle keywords are read under their former rate_limiter
// names too, scoped or not, so a cluster configuration written before the
// rename keeps its throttle.
func TestListenerThrottleReadsItsFormerNames(t *testing.T) {
	o, err := newCcfg(naming.Cluster, WithConfigData([]byte(`
[listener]
rate_limiter_rate = 30
rate_limiter_burst@`+hostname.Hostname()+` = 200
rate_limiter_expires = 2m
`)), WithVolatile(true))
	require.NoError(t, err)
	cfg := o.Config()
	assert.Equal(t, 30, cfg.GetInt(key.New("listener", "throttle_rate")))
	assert.Equal(t, 200, cfg.GetInt(key.New("listener", "throttle_burst")))
	expires := cfg.GetDuration(key.New("listener", "throttle_expires"))
	require.NotNil(t, expires)
	assert.Equal(t, 2*time.Minute, *expires)
}

// An ai.agent_url found invalid in a file changed by hand is reported among
// the issues, and left out of the cluster config: no agent is published.
func TestClusterConfigLeavesOutAnInvalidAIAgentURL(t *testing.T) {
	for value, want := range map[string]string{
		"https://ai.example.com:8090": "https://ai.example.com:8090",
		"http://ai.example.com:8090":  "",
	} {
		env := testhelper.Setup(t)
		require.NoError(t, os.MkdirAll(filepath.Join(env.Root, "etc"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(env.Root, "etc", "cluster.conf"), []byte("[ai]\nagent_url = "+value+"\n"), 0o600))
		cfg, err := getClusterConfig()
		require.NoError(t, err, value)
		assert.Equal(t, want, cfg.AI.AgentURL, value)
		hasIssue := false
		for _, issue := range cfg.Issues {
			hasIssue = hasIssue || strings.HasPrefix(issue, "ai agent_url")
		}
		assert.Equal(t, want == "", hasIssue, "%s: %v", value, cfg.Issues)
	}
}
