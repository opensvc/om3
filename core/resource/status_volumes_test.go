package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
)

// The vol objects of a resource status are its own in a copy: changing the
// copy leaves the status copied as it was.
func TestStatusDeepCopyVolumes(t *testing.T) {
	data, err := naming.ParsePath("ns1/vol/data")
	require.NoError(t, err)
	other, err := naming.ParsePath("ns1/vol/other")
	require.NoError(t, err)

	s := &Status{Volumes: naming.Paths{data}}
	c := s.DeepCopy()
	assert.Equal(t, s.Volumes, c.Volumes)
	c.Volumes[0] = other
	assert.Equal(t, "ns1/vol/data", s.Volumes[0].String())
}

// The vol objects of a resource are in its unstructured status too, which
// the table renderers read.
func TestStatusUnstructuredVolumes(t *testing.T) {
	data, err := naming.ParsePath("ns1/vol/data")
	require.NoError(t, err)
	s := &Status{Volumes: naming.Paths{data}}
	assert.Equal(t, naming.Paths{data}, s.Unstructured()["volumes"])
	assert.NotContains(t, (&Status{}).Unstructured(), "volumes")
}
