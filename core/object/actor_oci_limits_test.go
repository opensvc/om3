package object_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/drivers/rescontainerdocker"
	"github.com/opensvc/om3/v3/drivers/restaskdocker"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/capabilities"
)

// The limit_* keywords of the app resources are keywords of the oci
// containers and tasks too, read with the same units, and a task hands its
// own to the container of its runs.
func TestTheLimitKeywordsOfAnOCIContainerAndTask(t *testing.T) {
	testhelper.Setup(t)
	caps := []string{
		driver.NewID(driver.GroupContainer, "docker").Cap(),
		driver.NewID(driver.GroupTask, "docker").Cap(),
	}
	capabilities.Register(func(context.Context) ([]string, error) { return caps, nil })
	require.NoError(t, capabilities.Scan(t.Context()))
	p, err := naming.ParsePath("test/svc/web")
	require.NoError(t, err)
	o, err := object.NewSvc(p, object.WithVolatile(true), object.WithConfigData([]byte(`
[container#1]
type = docker
image = ghcr.io/opensvc/pause
limit_nofile = 1024
limit_cpu = 2m
limit_core = 1g

[task#1]
type = docker
image = ghcr.io/opensvc/pause
limit_nofile = 2048
limit_as = 1g
`)))
	require.NoError(t, err)
	seen := 0
	for _, r := range o.Resources() {
		switch r := r.(type) {
		case *rescontainerdocker.T:
			seen++
			require.NotNil(t, r.Limit.NoFile)
			assert.Equal(t, int64(1024), *r.Limit.NoFile)
			require.NotNil(t, r.Limit.CPU)
			assert.Equal(t, 2*time.Minute, *r.Limit.CPU)
			require.NotNil(t, r.Limit.Core)
			assert.Equal(t, int64(1<<30), *r.Limit.Core)
			assert.Nil(t, r.Limit.AS, "a limit not set is left to the engine")
		case *restaskdocker.T:
			seen++
			require.NotNil(t, r.Limit.NoFile)
			assert.Equal(t, int64(2048), *r.Limit.NoFile)
			require.NotNil(t, r.Limit.AS)
			assert.Equal(t, int64(1<<30), *r.Limit.AS)
		}
	}
	assert.Equal(t, 2, seen, "the container and the task")
}
