package resvol

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/provisioned"
)

// A volume is provisioned for a service once the service claimed it, the
// claim naming the node: an existing volume a service created again points
// at is not, and its provision claims it.
func TestAVolumeIsProvisionedForTheServiceClaimingIt(t *testing.T) {
	svc := naming.Path{Namespace: "ns1", Kind: naming.KindSvc, Name: "app"}
	other := naming.Relations{"ns1/svc/other@n1"}
	claimed := naming.Relations{"ns1/svc/other@n1", "ns1/svc/app@n2"}

	v, err := claimedBy(nil, svc)
	require.NoError(t, err)
	assert.Equal(t, provisioned.False, v, "no claim")

	v, err = claimedBy(other, svc)
	require.NoError(t, err)
	assert.Equal(t, provisioned.False, v, "claimed by another service")

	v, err = claimedBy(claimed, svc)
	require.NoError(t, err)
	assert.Equal(t, provisioned.True, v)
}
