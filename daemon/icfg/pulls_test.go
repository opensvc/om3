package icfg

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/plog"

	_ "github.com/opensvc/om3/v3/drivers/rescontainerdocker"
	_ "github.com/opensvc/om3/v3/drivers/rescontaineroci"
	_ "github.com/opensvc/om3/v3/drivers/rescontainerpodman"
)

func managerOf(t *testing.T, nodeConf, conf string) *Manager {
	t.Helper()
	testhelper.Setup(t)
	require.NoError(t, os.MkdirAll(rawconfig.Paths.Etc, 0o755))
	require.NoError(t, os.WriteFile(rawconfig.NodeConfigFile(), []byte(nodeConf), 0o600))
	caps := []string{
		driver.NewID(driver.GroupContainer, "docker").Cap(),
		driver.NewID(driver.GroupContainer, "podman").Cap(),
		driver.NewID(driver.GroupContainer, "oci").Cap(),
	}
	capabilities.Register(func(context.Context) ([]string, error) { return caps, nil })
	require.NoError(t, capabilities.Scan(t.Context()))
	p, err := naming.ParsePath("repro/svc/d")
	require.NoError(t, err)
	oc, err := object.NewConfigurer(p, object.WithConfigData([]byte(conf)), object.WithVolatile(true))
	require.NoError(t, err)
	return &Manager{path: p, configure: oc, log: plog.NewDefaultLogger()}
}

// The instance config publishes the image each container resource
// prefetches on the node, and follows the config of the node for the engine
// and the prefetch default they were evaluated with: an update of the node
// saying the same refreshes nothing, and one changing either does.
func TestSetPulls(t *testing.T) {
	m := managerOf(t, "[node]\noci = docker\n", `
[DEFAULT]
nodes = n1

[container#1]
image = nginx

[container#2]
type = podman
image = ghcr.io/opensvc/pause
image_prefetch = false

[app#1]
`)
	resources := m.getResources(m.configure.Config())
	m.setPulls(resources)
	require.NotNil(t, resources["container#1"].Pull)
	assert.Equal(t, resource.Pull{Engine: "docker", Image: "docker.io/library/nginx:latest"}, *resources["container#1"].Pull)
	assert.Nil(t, resources["container#2"].Pull, "a prefetch the resource disables")
	assert.Nil(t, resources["app#1"].Pull)

	assert.False(t, m.nodeConfigChangesPulls(node.Config{OCI: "docker", ImagePrefetch: true}))
	assert.False(t, m.nodeConfigChangesPulls(node.Config{OCI: "docker", ImagePrefetch: true, Env: "PRD"}),
		"another change of the node config")
	assert.True(t, m.nodeConfigChangesPulls(node.Config{OCI: "podman", ImagePrefetch: true}), "another engine")
	assert.True(t, m.nodeConfigChangesPulls(node.Config{OCI: "docker", ImagePrefetch: false}), "another default")
}

// An object with no container resource has nothing to evaluate again when
// the config of the node changes.
func TestSetPullsOfAnObjectWithNoContainer(t *testing.T) {
	m := managerOf(t, "", "[DEFAULT]\nnodes = n1\n\n[app#1]\n")
	resources := m.getResources(m.configure.Config())
	m.setPulls(resources)
	assert.Nil(t, resources["app#1"].Pull)
	assert.False(t, m.nodeConfigChangesPulls(node.Config{OCI: "podman"}))
}
