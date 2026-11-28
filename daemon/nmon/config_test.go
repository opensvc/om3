package nmon

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/plog"
)

// The pools are loaded every node.load_pool_interval, 60s when the node
// config sets none, or sets a duration that is no interval, and 1s at least,
// for a lower one written by hand, which the keyword refuses.
func TestLoadPoolInterval(t *testing.T) {
	for conf, want := range map[string]time.Duration{
		"":                             node.DefaultLoadPoolInterval,
		"load_pool_interval = 5m\n":    5 * time.Minute,
		"load_pool_interval = 0s\n":    node.DefaultLoadPoolInterval,
		"load_pool_interval = 90s\n":   90 * time.Second,
		"load_pool_interval = 500ms\n": node.MinLoadPoolInterval,
	} {
		testhelper.Setup(t)
		t.Cleanup(func() { rawconfig.Load(map[string]string{}) })
		require.NoError(t, os.WriteFile(rawconfig.NodeConfigFile(), []byte("[node]\n"+conf), 0600))
		n, err := object.NewNode(object.WithVolatile(true))
		require.NoError(t, err)

		m := &Manager{log: plog.NewDefaultLogger()}
		m.config = n.MergedConfig()
		cfg := m.getNodeConfig()
		assert.Equal(t, want, cfg.LoadPoolInterval, "%q", conf)

		m.loadPoolInterval.Store(int64(cfg.LoadPoolInterval))
		assert.Equal(t, want, m.getLoadPoolInterval(), "%q", conf)
	}
	assert.Equal(t, node.DefaultLoadPoolInterval, (&Manager{}).getLoadPoolInterval(), "before the config is loaded")
}

// The node config publishes the engine of the oci resources and the
// prefetch default of the node, read from its configuration merged over the
// cluster one: the instance configs evaluate the images they prefetch with
// them, and are refreshed when either changes.
func TestNodeConfigSaysTheEngineAndThePrefetchDefault(t *testing.T) {
	for name, tc := range map[string]struct {
		node     string
		cluster  string
		oci      string
		prefetch bool
	}{
		"the defaults":                    {oci: "docker", prefetch: true},
		"set by the node":                 {node: "[node]\noci = podman\nimage_prefetch = false\n", oci: "podman"},
		"set by the cluster":              {cluster: "[node]\nimage_prefetch = false\n", oci: "docker"},
		"the node overriding the cluster": {node: "[node]\nimage_prefetch = true\n", cluster: "[node]\nimage_prefetch = false\n", oci: "docker", prefetch: true},
	} {
		t.Run(name, func(t *testing.T) {
			testhelper.Setup(t)
			t.Cleanup(func() { rawconfig.Load(map[string]string{}) })
			require.NoError(t, os.WriteFile(rawconfig.NodeConfigFile(), []byte(tc.node), 0600))
			require.NoError(t, os.WriteFile(rawconfig.ClusterConfigFile(), []byte("[cluster]\nname = cluster1\n"+tc.cluster), 0600))
			n, err := object.NewNode(object.WithVolatile(true))
			require.NoError(t, err)
			m := &Manager{log: plog.NewDefaultLogger()}
			m.config = n.MergedConfig()
			cfg := m.getNodeConfig()
			assert.Equal(t, tc.oci, cfg.OCI)
			assert.Equal(t, tc.prefetch, cfg.ImagePrefetch)
		})
	}
	cfg := node.Config{OCI: "docker", ImagePrefetch: true}
	assert.True(t, cfg.Equal(node.Config{OCI: "docker", ImagePrefetch: true}))
	assert.False(t, cfg.Equal(node.Config{OCI: "podman", ImagePrefetch: true}))
	assert.False(t, cfg.Equal(node.Config{OCI: "docker"}))
}
