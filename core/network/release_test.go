package network_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/ipam"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/core/rawconfig"
	// The lo and default networks are implicit, and their drivers too, as
	// the daemon registers them.
	_ "github.com/opensvc/om3/v3/drivers/networkbridge"
	_ "github.com/opensvc/om3/v3/drivers/networklan"
	_ "github.com/opensvc/om3/v3/drivers/networklo"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/hostname"
)

// The reservations of the resources an object no longer has are released,
// and nothing else: its other resources, and the other objects, keep theirs.
func TestReleaseRemoved(t *testing.T) {
	testhelper.Setup(t)
	require.NoError(t, os.WriteFile(rawconfig.NodeConfigFile(), []byte("[network#san]\ntype = lan\nnetwork = fd01::/64\nranges = fd01::5:0/120\n"), 0600))
	nw, _, err := network.Lookup("san")
	require.NoError(t, err)
	require.NotNil(t, nw)
	a, err := network.NewAllocator(nw, hostname.Hostname())
	require.NoError(t, err)

	pa, _ := naming.ParsePath("ns1/svc/a")
	pb, _ := naming.ParsePath("ns1/svc/b")
	for _, key := range []string{ipam.Key(pa, "ip#1"), ipam.Key(pa, "ip#2"), ipam.Key(pb, "ip#2")} {
		_, err := a.Allocate(key)
		require.NoError(t, err)
	}

	n, err := network.ReleaseRemoved(pa, map[string]bool{"ip#1": true})
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	for key, held := range map[string]bool{ipam.Key(pa, "ip#1"): true, ipam.Key(pa, "ip#2"): false, ipam.Key(pb, "ip#2"): true} {
		ip, err := a.Allocated(key)
		require.NoError(t, err)
		assert.Equal(t, held, ip != nil, key)
	}
}
