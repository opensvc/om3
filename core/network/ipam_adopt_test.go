package network

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/ipam"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/testhelper"
)

// The addresses the resources of the node hold are the ones their status
// reports, but for an unprovisioned resource: a lan address a follower of an
// unprovision released is still in the configuration, and reported, until
// the leader removes it, and a network setup in between took it back.
func TestLocalReservationsSkipUnprovisioned(t *testing.T) {
	testhelper.Setup(t)
	p, err := naming.ParsePath("ns1/svc/web")
	require.NoError(t, err)
	cf := filepath.Join(rawconfig.Paths.EtcNs, "ns1", "svc", "web.conf")
	require.NoError(t, os.MkdirAll(filepath.Dir(cf), 0o700))
	require.NoError(t, os.WriteFile(cf, []byte{}, 0o600))

	withAddr := func(addr string, state provisioned.T) resource.Status {
		return resource.Status{
			Info:          map[string]any{ipAddrInfoKey: addr},
			IsProvisioned: resource.ProvisionStatus{State: state},
		}
	}
	st := instance.Status{Resources: instance.ResourceStatuses{
		"ip#0": withAddr("fd01::5:1", provisioned.False),
		"ip#1": withAddr("fd01::5:2", provisioned.True),
		"ip#2": withAddr("fd01::5:3", provisioned.NotApplicable),
	}}
	b, err := json.Marshal(st)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(p.VarDir(), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(p.VarDir(), "status.json"), b, 0o600))

	l, err := localReservations()
	require.NoError(t, err)
	keys := make([]string, 0, len(l))
	for _, r := range l {
		keys = append(keys, r.Key)
	}
	assert.ElementsMatch(t, []string{ipam.Key(p, "ip#1"), ipam.Key(p, "ip#2")}, keys)
}
