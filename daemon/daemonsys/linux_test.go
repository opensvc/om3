//go:build linux

package daemonsys

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInCgroup(t *testing.T) {
	unit := "/system.slice/opensvc-server.service"
	cases := map[string]struct {
		content string
		want    bool
	}{
		"cgroup v1, the unit": {
			content: "11:cpuset:/\n2:cpu,cpuacct:/system.slice/opensvc-server.service\n1:name=systemd:/system.slice/opensvc-server.service\n",
			want:    true,
		},
		"cgroup v1, a user session": {
			content: "2:cpu,cpuacct:/user.slice\n1:name=systemd:/user.slice/user-0.slice/session-3.scope\n",
			want:    false,
		},
		"cgroup v1, the unit in a controller only": {
			content: "2:cpu,cpuacct:/system.slice/opensvc-server.service\n1:name=systemd:/user.slice/user-0.slice/session-3.scope\n",
			want:    false,
		},
		"cgroup v2, the unit": {
			content: "0::/system.slice/opensvc-server.service\n",
			want:    true,
		},
		"cgroup v2, under the unit": {
			content: "0::/system.slice/opensvc-server.service/control\n",
			want:    true,
		},
		"cgroup v2, a process group of an object": {
			content: "0::/opensvc.slice/opensvc-ns.root.slice/opensvc-ns.root-svc.foo.slice\n",
			want:    false,
		},
		"hybrid, the unit": {
			content: "1:name=systemd:/system.slice/opensvc-server.service\n0::/system.slice/opensvc-server.service\n",
			want:    true,
		},
		"another unit": {
			content: "0::/system.slice/opensvc-server.service.d\n",
			want:    false,
		},
		"a unit of the same name of a user manager": {
			content: "0::/user.slice/user-0.slice/user@0.service/app.slice/opensvc-server.service\n",
			want:    false,
		},
		"cgroup v1, a unit of the same name of a user manager": {
			content: "1:name=systemd:/user.slice/user-0.slice/user@0.service/opensvc-server.service\n",
			want:    false,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, inCgroup([]byte(c.content), unit))
		})
	}
}
