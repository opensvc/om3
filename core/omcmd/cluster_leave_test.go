package omcmd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/key"
)

// TestKeptConfigSurvivesTheLeave pins that the leave keeps the [collector]
// section of the cluster config, and the node.uuid and node.prkey of the
// node config, and nothing else of either: the cluster keys are the ones of
// the cluster left, the daemon bootstraps its own.
func TestKeptConfigSurvivesTheLeave(t *testing.T) {
	env := testhelper.Setup(t)
	t.Cleanup(func() { rawconfig.Load(map[string]string{}) })
	ccfg := "[cluster]\nname = left\nnodes = node1 node2\nsecret = s3cr3t\n\n" +
		"[collector]\nurl = https://collector.example.com\ntimeout = 10s\n\n" +
		"[node]\nenv = PRD\nprkey = 0x0000000000000001\n"
	require.NoError(t, os.WriteFile(rawconfig.ClusterConfigFile(), []byte(ccfg), 0600))
	ncfg := "[node]\nuuid = uuid-old\nprkey = 0x00000000000000ab\napp = app1\n"
	require.NoError(t, os.WriteFile(rawconfig.NodeConfigFile(), []byte(ncfg), 0600))

	kept := readKeptConfig()
	assert.Equal(t, map[string]string{"url": "https://collector.example.com", "timeout": "10s"}, kept.collector)
	assert.Equal(t, map[string]string{"uuid": "uuid-old", "prkey": "0x00000000000000ab"}, kept.node,
		"the node keys of the node config, not the ones of the cluster config")

	// The leave moves the config aside.
	require.NoError(t, os.Rename(rawconfig.Paths.Etc, env.Root+"/etc.left"))
	require.NoError(t, rawconfig.CreateMandatoryDirectories())

	require.NoError(t, kept.write())

	c, err := object.NewCluster(object.WithVolatile(true))
	require.NoError(t, err)
	assert.Equal(t, kept.collector, c.Config().SectionMap("collector"))
	assert.Empty(t, c.Config().SectionMap("cluster"), "the cluster keys of the cluster left")
	assert.Empty(t, c.Config().SectionMap("node"), "the node keys of the cluster config")

	n, err := object.NewNode()
	require.NoError(t, err)
	assert.Equal(t, kept.node, n.Config().SectionMap("node"))
	assert.Equal(t, "uuid-old", n.Config().GetString(key.Parse("node.uuid")))
	assert.Equal(t, "0x00000000000000ab", n.Config().GetString(key.Parse("node.prkey")))
}

// TestKeptConfigNone pins that a node with none of the kept settings leaves
// with no config written for them.
func TestKeptConfigNone(t *testing.T) {
	testhelper.Setup(t)
	t.Cleanup(func() { rawconfig.Load(map[string]string{}) })

	kept := readKeptConfig()
	assert.Empty(t, kept.collector)
	assert.Empty(t, kept.node)

	require.NoError(t, kept.write())
	assert.NoFileExists(t, rawconfig.ClusterConfigFile())
	assert.NoFileExists(t, rawconfig.NodeConfigFile())
}

// TestKeptConfigKeepsTheNodeKeysOnARefusedCollectorSection pins that a
// [collector] section the cluster config refuses, as one holding a scoped
// key, is reported without losing the node keys.
func TestKeptConfigKeepsTheNodeKeysOnARefusedCollectorSection(t *testing.T) {
	testhelper.Setup(t)
	t.Cleanup(func() { rawconfig.Load(map[string]string{}) })

	kept := keptConfig{
		collector: map[string]string{"timeout@node1": "10s"},
		node:      map[string]string{"uuid": "uuid-old", "prkey": "0x00000000000000ab"},
	}
	assert.ErrorContains(t, kept.write(), "keep the collector settings")

	n, err := object.NewNode()
	require.NoError(t, err)
	assert.Equal(t, "uuid-old", n.Config().GetString(key.Parse("node.uuid")))
	assert.Equal(t, "0x00000000000000ab", n.Config().GetString(key.Parse("node.prkey")))
}
