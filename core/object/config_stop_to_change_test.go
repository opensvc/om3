package object

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/testhelper"
)

// A change a resource up on a node would leak by is refused: a keyword
// saying what it holds, as written or through what it references, its type,
// or the removal of its section. Other changes, and changes of a resource
// known to be stopped, are not.
func TestCheckStopToChange(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := SetClusterConfig()
	require.NoError(t, err)

	p, err := naming.ParsePath("test/svc/foo")
	require.NoError(t, err)
	cfgOf := func(s string) *xconfig.T {
		t.Helper()
		o, err := NewSvc(p, WithConfigData([]byte(s)), WithVolatile(true))
		require.NoError(t, err)
		return o.Config()
	}
	statuses := func(byNode map[string]status.T) map[string]*instance.Status {
		m := make(map[string]*instance.Status)
		for nodename, st := range byNode {
			m[nodename] = &instance.Status{Resources: instance.ResourceStatuses{"fs#1": resource.Status{Status: st}}}
		}
		return m
	}
	from := cfgOf("[DEFAULT]\nnodes = node1\n[env]\nd = a\n[fs#1]\ntype = ext4\ndev = /dev/sdb\nmnt = /srv/{env.d}\n")
	up := statuses(map[string]status.T{"node1": status.Up})

	for _, tc := range []struct {
		name string
		to   string
		want string
	}{
		{"a mount point", "[DEFAULT]\nnodes = node1\n[env]\nd = a\n[fs#1]\ntype = ext4\ndev = /dev/sdb\nmnt = /srv/b\n", `change fs#1.mnt from "/srv/a" to "/srv/b": up on node1`},
		{"a reference of the mount point", "[DEFAULT]\nnodes = node1\n[env]\nd = b\n[fs#1]\ntype = ext4\ndev = /dev/sdb\nmnt = /srv/{env.d}\n", `change fs#1.mnt from "/srv/a" to "/srv/b"`},
		{"the type", "[DEFAULT]\nnodes = node1\n[env]\nd = a\n[fs#1]\ntype = xfs\ndev = /dev/sdb\nmnt = /srv/{env.d}\n", `change fs#1.type from "ext4" to "xfs"`},
		{"the section", "[DEFAULT]\nnodes = node1\n[env]\nd = a\n", "delete fs#1: up on node1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckStopToChange(p.Kind, from, cfgOf(tc.to), up)
			assert.ErrorIs(t, err, ErrStopToChange)
			assert.ErrorContains(t, err, tc.want)
		})
	}

	edited := cfgOf("[DEFAULT]\nnodes = node1\n[env]\nd = a\n[fs#1]\ntype = ext4\ndev = /dev/sdb\nmnt = /srv/{env.d}\nmnt_opt = noatime\ncomment = data\n")
	assert.NoError(t, CheckStopToChange(p.Kind, from, edited, up), "a keyword saying nothing of what it holds")
	assert.NoError(t, CheckStopToChange(p.Kind, from, from, up), "no change")
	assert.NoError(t, CheckStopToChange(p.Kind, nil, edited, up), "an object created changes nothing")

	removed := cfgOf("[DEFAULT]\nnodes = node1\n[env]\nd = a\n")
	for _, st := range []status.T{status.Down, status.StandbyDown, status.NotApplicable} {
		assert.NoError(t, CheckStopToChange(p.Kind, from, removed, statuses(map[string]status.T{"node1": st})), st.String())
	}
	assert.Error(t, CheckStopToChange(p.Kind, from, removed, statuses(map[string]status.T{"node1": status.Undef})), "a status not known to be stopped is not stopped")
	assert.ErrorContains(t, CheckStopToChange(p.Kind, from, removed, statuses(map[string]status.T{"node1": status.Down, "node2": status.StandbyUp})), "stdby up on node2")
}
