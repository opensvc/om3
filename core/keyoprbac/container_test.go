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

// The namespaces a container or a task joins are the engine's private ones,
// or the ones of a container of the object, for a user holding no root
// grant. Any other value goes to the engine as it is, and reaches what the
// object does not own: the namespaces of the node, by name or by path, or the
// ones of a container of another object.
func TestTheNamespacesJoinedAreTheObjects(t *testing.T) {
	admin := rbac.Grants{rbac.NewGrant(rbac.RoleAdmin, "ns1")}
	for _, section := range []string{"container#1", "task#1"} {
		for _, option := range []string{"netns", "pidns", "ipcns", "utsns", "userns"} {
			for _, value := range []string{"", "none", "container#0", "container#pause", " container#0 "} {
				assert.NoErrorf(t, Denied(admin, naming.KindSvc, section, option, value, none),
					"%s.%s=%q", section, option, value)
			}
			for _, value := range []string{
				"host",
				"container:othersvc.container.0",
				"ns:/proc/1/ns/net",
				"private",
				"auto",
				"keep-id",
				"fs#1",
				"container#",
				"container#*",
				"container#0 host",
			} {
				assert.Errorf(t, Denied(admin, naming.KindSvc, section, option, value, none),
					"%s.%s=%q", section, option, value)
			}
		}
	}
	root := rbac.Grants{rbac.GrantRoot}
	assert.NoError(t, Denied(root, naming.KindSvc, "container#1", "pidns", "host", none))
	assert.NoError(t, Denied(root, naming.KindSvc, "container#1", "netns", "ns:/proc/1/ns/net", none))

	// The address of an ip resource lives in the namespace of a container of
	// the object, never another.
	assert.Error(t, Denied(admin, naming.KindSvc, "ip#1", "netns", "container:othersvc.container.0", none))
	assert.Error(t, Denied(admin, naming.KindSvc, "ip#1", "netns", "ns:/proc/1/ns/net", none))
}

// The apparmor profile of a container, whatever its value, unconfined or
// a profile loaded on the node, needs the root grant.
func TestContainerAppArmorProfileNeedsRoot(t *testing.T) {
	admin := rbac.Grants{rbac.NewGrant(rbac.RoleAdmin, "ns1")}
	root := rbac.Grants{rbac.GrantRoot}
	for _, v := range []string{"unconfined", "docker-default", "my-profile"} {
		assert.Errorf(t, Denied(admin, naming.KindSvc, "container#1", "apparmor_profile", v, none), "admin sets %s", v)
		assert.NoErrorf(t, Denied(root, naming.KindSvc, "container#1", "apparmor_profile", v, none), "root sets %s", v)
	}
}

// The limit_* keywords of a container and of a task are the ulimits the
// engine sets on its processes, which the administrator of a namespace sets
// without the root grant the --ulimit of run_args requires. The limits on the
// core dumps and on the locked memory are the exception, whatever the value.
func TestTheLimitsOfAContainerAreOpenButCoreAndMemlock(t *testing.T) {
	root := rbac.Grants{rbac.GrantRoot}
	for _, section := range []string{"container#1", "task#1"} {
		for _, option := range []string{"limit_as", "limit_cpu", "limit_data", "limit_fsize", "limit_nofile", "limit_nproc", "limit_rss", "limit_stack", "limit_vmem"} {
			for _, value := range []string{"1024", "1g", "unlimited", ""} {
				assert.NoErrorf(t, Denied(noGrant, naming.KindSvc, section, option, value, none), "%s.%s=%s", section, option, value)
			}
			assert.Emptyf(t, Doc(naming.KindSvc, "container", option), "%s documents a grant", option)
		}
		for _, option := range []string{"limit_core", "limit_memlock"} {
			for _, value := range []string{"0", "1g", "unlimited"} {
				err := Denied(noGrant, naming.KindSvc, section, option, value, none)
				assert.EqualErrorf(t, err, "the core dump and locked memory limits of a container require the root grant", "%s.%s=%s", section, option, value)
				assert.NoErrorf(t, Denied(root, naming.KindSvc, section, option, value, none), "%s.%s=%s as root", section, option, value)
			}
		}
		assert.Error(t, Denied(noGrant, naming.KindSvc, section, "run_args", "--ulimit nofile=1024:1024", none), "run_args stays root's")
	}
}
