package object_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/capabilities"

	_ "github.com/opensvc/om3/v3/drivers/rescontainerdocker"
	_ "github.com/opensvc/om3/v3/drivers/rescontaineroci"
	_ "github.com/opensvc/om3/v3/drivers/rescontainerpodman"
)

// A container section naming no type runs the oci driver of its group, and
// so the engine of the node, as one naming type = oci: the node choosing
// docker, both are docker, whatever the oci virtual driver prefers.
func TestAContainerNamingNoTypeRunsTheEngineOfTheNode(t *testing.T) {
	testhelper.Setup(t)
	require.NoError(t, os.MkdirAll(rawconfig.Paths.Etc, 0o755))
	require.NoError(t, os.WriteFile(rawconfig.NodeConfigFile(), []byte("[node]\noci = docker\n"), 0o600))
	// A node with both engines, where the oci virtual driver prefers
	// podman.
	caps := []string{
		driver.NewID(driver.GroupContainer, "docker").Cap(),
		driver.NewID(driver.GroupContainer, "podman").Cap(),
		driver.NewID(driver.GroupContainer, "oci").Cap(),
	}
	capabilities.Register(func(context.Context) ([]string, error) { return caps, nil })
	require.NoError(t, capabilities.Scan(t.Context()))
	p, err := naming.ParsePath("test/svc/web")
	require.NoError(t, err)
	o, err := object.NewSvc(p, object.WithVolatile(true), object.WithConfigData([]byte(`
[container#0]
image = ghcr.io/opensvc/pause

[container#1]
type = oci
image = ghcr.io/opensvc/pause
`)))
	require.NoError(t, err)
	drivers := make(map[string]string)
	for _, r := range o.Resources() {
		drivers[r.RID()] = r.Manifest().DriverID.String()
	}
	assert.Equal(t, map[string]string{"container#0": "container.docker", "container#1": "container.docker"}, drivers)
}
