package object

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/testhelper"

	_ "github.com/opensvc/om3/v3/drivers/resfsflag"
)

// A section of a driver this agent does not have is not configured, and is
// recorded to be reported by the status: a driver of a known group, and a
// group this agent does not know. A subset section is no resource, and
// neither is a section without the shape of a resource id, as the empty node
// section a past bug left in object configurations.
func TestConfigureResourcesRecordsTheUnsupportedSections(t *testing.T) {
	testhelper.Setup(t)
	p, err := naming.ParsePath("test/svc/ghost")
	require.NoError(t, err)
	o, err := New(p, WithConfigData([]byte(`[DEFAULT]
nodes = *

[disk#1]
type = vxdg

[expose#1]
type = envoy

[fs#1]
type = flag
subset = g1

[subset#g1]
parallel = false

[node]
`)), WithVolatile(true))
	require.NoError(t, err)
	a := o.(*svc)
	a.ConfigureResources()
	assert.Equal(t, map[string]string{
		"disk#1":   "disk.vxdg",
		"expose#1": "expose.envoy",
	}, a.unsupportedResources())
	require.Len(t, a.Resources(), 1)
	assert.Equal(t, "fs#1", a.Resources()[0].RID())
}

// The status of an unsupported resource warns, which raises the overall
// status, and is optional, which keeps it out of the availability.
func TestUnsupportedResourceStatus(t *testing.T) {
	s := unsupportedResourceStatus("disk.vxdg")
	assert.Equal(t, status.NotApplicable, s.Status)
	assert.True(t, s.IsOptional)
	assert.Equal(t, provisioned.NotApplicable, s.IsProvisioned.State)
	require.Len(t, s.Log, 1)
	assert.Equal(t, resource.WarnLevel, s.Log[0].Level)
	assert.Contains(t, s.Log[0].Message, "disk.vxdg driver is not supported")
}
