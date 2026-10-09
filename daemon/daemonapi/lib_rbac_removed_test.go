package daemonapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/xconfig"
)

// fakeInstanceStatuses has ip#1 of the object in the given status on each
// node.
func fakeInstanceStatuses(t *testing.T, byNode map[string]status.T) {
	t.Helper()
	prev := instanceStatusesOf
	t.Cleanup(func() { instanceStatusesOf = prev })
	instanceStatusesOf = func(naming.Path) map[string]*instance.Status {
		m := make(map[string]*instance.Status)
		for nodename, st := range byNode {
			m[nodename] = &instance.Status{Resources: map[string]resource.Status{"ip#1": {Status: st}}}
		}
		return m
	}
}

func cfgOf(t *testing.T, s string) *xconfig.T {
	t.Helper()
	cfg, err := xconfig.NewObject("", []byte(s))
	require.NoError(t, err)
	return cfg
}

// A section is taken away only once its resource is stopped everywhere: what
// it holds on a node stays there otherwise, out of the reach of the user who
// removed it.
func TestRemovingTheSectionOfARunningResource(t *testing.T) {
	p, _ := naming.ParsePath("test/svc/foo")
	from := cfgOf(t, "[DEFAULT]\nnodes = n1 n2\n[ip#1]\ntype = host\nnetwork = san\n")
	to := cfgOf(t, "[DEFAULT]\nnodes = n1 n2\n")

	fakeInstanceStatuses(t, map[string]status.T{"n1": status.Up, "n2": status.Down})
	err := removedResourceRbac(p, from, to)
	assert.ErrorContains(t, err, "delete ip#1: the resource is not stopped (up on n1)")

	fakeInstanceStatuses(t, map[string]status.T{"n1": status.Undef, "n2": status.StandbyUp})
	assert.Error(t, removedResourceRbac(p, from, to), "a status not known to be stopped is not stopped")

	fakeInstanceStatuses(t, map[string]status.T{"n1": status.Down, "n2": status.StandbyDown})
	assert.NoError(t, removedResourceRbac(p, from, to))

	fakeInstanceStatuses(t, map[string]status.T{"n1": status.NotApplicable})
	assert.NoError(t, removedResourceRbac(p, from, to))

	fakeInstanceStatuses(t, map[string]status.T{"n1": status.Up})
	assert.NoError(t, removedResourceRbac(p, from, from), "a section kept is no removal")
	assert.NoError(t, removedResourceRbac(p, nil, to), "an object created removes nothing")
	edited := cfgOf(t, "[DEFAULT]\nnodes = n1 n2\n[ip#1]\ntype = host\nnetwork = san\ncomment = portal\n")
	assert.NoError(t, removedResourceRbac(p, from, edited), "an edit of a running resource")
	withEnv := cfgOf(t, "[DEFAULT]\nnodes = n1 n2\n[env]\na = 1\n[ip#1]\ntype = host\nnetwork = san\n")
	assert.NoError(t, removedResourceRbac(p, withEnv, from), "a section that is no resource")
}
