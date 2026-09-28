package keyoprbac

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/rbac"
)

// The container and task keywords deciding what of the node a container
// reaches need the root grant, as they did in v2. The values reaching nothing
// of the node stay open.
func TestContainerRulesPortedFromV2(t *testing.T) {
	admin := rbac.Grants{rbac.NewGrant(rbac.RoleAdmin, "ns1")}
	for _, section := range []string{"container#1", "task#1"} {
		for _, tc := range []struct {
			option, value string
			denied        bool
		}{
			{"privileged", "true", true},
			{"privileged", "yes", true},
			{"privileged", "false", false},
			{"netns", "host", true},
			{"netns", "container#0", false},
			{"devices", "/dev/fuse:/dev/fuse", true},
			{"volume_mounts", "/etc:/mnt", true},
			{"volume_mounts", "vol1:/data", false},
		} {
			err := Denied(admin, naming.KindSvc, section, tc.option, tc.value, none)
			if tc.denied {
				assert.Errorf(t, err, "%s.%s=%s", section, tc.option, tc.value)
			} else {
				assert.NoErrorf(t, err, "%s.%s=%s", section, tc.option, tc.value)
			}
		}
	}
	assert.Error(t, Denied(admin, naming.KindSvc, "ip#1", "netns", "host", none))
	assert.NoError(t, Denied(admin, naming.KindSvc, "ip#1", "netns", "container#0", none))
}
