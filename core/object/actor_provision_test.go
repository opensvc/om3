package object

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
)

// An instance is running before a provision when the resources it had
// provisioned were up: a resource being added, not provisioned yet, does not
// make it standing by, and a disk up on a node standing by does not make it
// running.
func TestRunningBeforeProvision(t *testing.T) {
	res := func(s status.T, prov provisioned.T) resource.Status {
		return resource.Status{Status: s, IsProvisioned: resource.ProvisionStatus{State: prov}}
	}
	for _, tc := range []struct {
		name      string
		resources instance.ResourceStatuses
		want      bool
	}{
		{"running", instance.ResourceStatuses{
			"fs#1": res(status.Up, provisioned.True),
			"fs#2": res(status.Up, provisioned.True),
		}, true},
		{"running, a resource being added", instance.ResourceStatuses{
			"fs#1": res(status.Up, provisioned.True),
			"fs#2": res(status.Down, provisioned.False),
		}, true},
		{"standing by, a disk up", instance.ResourceStatuses{
			"disk#1":      res(status.Up, provisioned.True),
			"container#1": res(status.Down, provisioned.True),
		}, false},
		{"down", instance.ResourceStatuses{
			"fs#1": res(status.Down, provisioned.True),
		}, false},
		{"nothing provisioned", instance.ResourceStatuses{
			"fs#1": res(status.Down, provisioned.False),
		}, false},
		{"a sync up does not make it running", instance.ResourceStatuses{
			"fs#1":   res(status.Down, provisioned.True),
			"sync#1": res(status.Up, provisioned.True),
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, runningBeforeProvision(instance.Status{Resources: tc.resources}))
		})
	}
}
