package resvol

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resourceid"
)

// A volume resource reports the vol object it is, in the namespace of its
// object: the one named, else the default name Volume opens too.
func TestVolumeList(t *testing.T) {
	rid, err := resourceid.Parse("volume#1")
	require.NoError(t, err)
	path, err := naming.ParsePath("ns1/svc/web")
	require.NoError(t, err)

	r := &T{Name: "data", Path: path}
	r.ResourceID = rid
	assert.Equal(t, "ns1/vol/data", r.VolumeList(context.Background()).String())

	r.Name = ""
	assert.Equal(t, "ns1/vol/web-vol-1", r.VolumeList(context.Background()).String())
}
