package rescontainerkvm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/netrule"
	"github.com/opensvc/om3/v3/drivers/shared/netwarden"
)

func TestNetKeywordsTakeThePorts(t *testing.T) {
	kws := New().(*T).Manifest().Keywords()
	seen := 0
	for _, kw := range kws {
		if kw.Option != netwarden.Option {
			continue
		}
		seen++
		require.NotNil(t, kw.Validate)
		assert.NoError(t, kw.Validate("allow object ./svc/db port 5432 proto tcp\ndeny any port 8000-8099 proto udp"),
			"the filter of the interfaces of a guest knows the ports")
		assert.Error(t, kw.Validate("allow any port 80"))
		assert.True(t, kw.Scopable)
	}
	assert.Equal(t, 1, seen)
}

func TestNetTargetIsTheInterfacesOmMakes(t *testing.T) {
	r := New().(*T)
	r.Path = naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "vm"}
	require.NoError(t, r.SetRID("container#1"))
	r.Network = []string{"backend", "lan"}
	r.Net = netwarden.T{Rules: "allow node n2 port 22 proto tcp"}
	net, target := r.NetRules()
	assert.Equal(t, r.Net, net)
	assert.Equal(t, netrule.BackendNetdev, target.Backend)
	assert.Nil(t, target.PG, "the cgroup of qemu carries nothing of the traffic of the guest")
	require.Len(t, target.Devs, 2)
	assert.Equal(t, r.netDevs(), target.Devs)
	assert.NotEqual(t, target.Devs[0], target.Devs[1])
	rec := net.Record(target)
	assert.Equal(t, netrule.BackendNetdev, rec.Backend)
	assert.Equal(t, target.Devs, rec.Devs)
	assert.Equal(t, "test", rec.Namespace)
	_, err := rec.Rules()
	assert.NoError(t, err, "a port is a rule of this backend")
}

func TestNetIssueOfAGuestWithNoInterfaceOfOm(t *testing.T) {
	r := New().(*T)
	r.Path = naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "vm"}
	require.NoError(t, r.netIssue(), "no rule, nothing to refuse")

	r.Net = netwarden.T{Rules: "deny any port 25 proto tcp\nallow any"}
	assert.EqualError(t, r.netIssue(),
		"netwarden: net_rules filters the interfaces om makes for the guest, the ones of the network keyword, and it names none")
	assert.Error(t, r.NetLiveIssue(context.Background()))

	// The start is refused before anything is made.
	_, err := r.networkUp(context.Background())
	assert.ErrorContains(t, err, "the ones of the network keyword")
}
