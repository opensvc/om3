package object

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/hostname"

	_ "github.com/opensvc/om3/v3/drivers/resdiskloop"
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
		{"the resource disabled", "[DEFAULT]\nnodes = node1\n[env]\nd = a\n[fs#1]\ntype = ext4\ndev = /dev/sdb\nmnt = /srv/{env.d}\ndisable = true\n", "disable fs#1: up on node1"},
		{"the object disabled", "[DEFAULT]\nnodes = node1\ndisable = true\n[env]\nd = a\n[fs#1]\ntype = ext4\ndev = /dev/sdb\nmnt = /srv/{env.d}\n", "disable fs#1: up on node1"},
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
	unset := cfgOf("[DEFAULT]\nnodes = node1\n[env]\nd = a\n[fs#1]\ntype = ext4\ndev = /dev/sdb\n")
	assert.NoError(t, CheckStopToChange(p.Kind, unset, from, up), "a value set where none was")
	assert.NoError(t, CheckStopToChange(p.Kind, nil, edited, up), "an object created changes nothing")

	removed := cfgOf("[DEFAULT]\nnodes = node1\n[env]\nd = a\n")
	for _, st := range []status.T{status.Down, status.StandbyDown, status.NotApplicable} {
		assert.NoError(t, CheckStopToChange(p.Kind, from, removed, statuses(map[string]status.T{"node1": st})), st.String())
	}
	err = CheckStopToChange(p.Kind, from, removed, statuses(map[string]status.T{"node1": status.Undef}))
	assert.ErrorIs(t, err, ErrStopToChange, "a status not known to be stopped is not stopped")
	assert.ErrorContains(t, err, "status not known (undef) on node1")
	assert.ErrorContains(t, err, "force the change once sure it holds nothing")

	// Undef for an error of its configuration, holding nothing, the
	// resource is stopped: the change is what fixes it.
	misconfigured := map[string]*instance.Status{"node1": {Resources: instance.ResourceStatuses{
		"fs#1": resource.Status{Status: status.Undef, ConfigError: "mode is not one of the modes of the network"},
	}}}
	assert.NoError(t, CheckStopToChange(p.Kind, from, removed, misconfigured))
	assert.NotContains(t, CheckStopToChange(p.Kind, from, removed, up).Error(), "force", "a resource up is stopped first")
	assert.ErrorContains(t, CheckStopToChange(p.Kind, from, removed, statuses(map[string]status.T{"node1": status.Down, "node2": status.StandbyUp})), "stdby up on node2")
}

// A task holds something on a node while it runs, which its stop ends: its
// status says how its last run went, and a task whose last run succeeded is
// up without holding anything. Its section is removed then, not while it
// runs.
func TestCheckStopToChangeTask(t *testing.T) {
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
	from := cfgOf("[DEFAULT]\nnodes = node1\n[task#1]\ntype = host\ncommand = echo TASK1\ncheck = last_run\n")
	removed := cfgOf("[DEFAULT]\nnodes = node1\n")
	lastRunUp := map[string]*instance.Status{
		"node1": {Resources: instance.ResourceStatuses{"task#1": resource.Status{Status: status.Up}}},
	}
	assert.NoError(t, CheckStopToChange(p.Kind, from, removed, lastRunUp), "a task up from its last run, not running")

	running := map[string]*instance.Status{
		"node1": {
			Resources: instance.ResourceStatuses{"task#1": resource.Status{Status: status.Up}},
			Running:   resource.RunningInfoList{{RID: "task#1"}},
		},
	}
	err = CheckStopToChange(p.Kind, from, removed, running)
	assert.ErrorIs(t, err, ErrStopToChange)
	assert.ErrorContains(t, err, "delete task#1: running on node1")
}

// With no status cache, the status of the instance is evaluated, so a local
// change is judged on a status known.
func TestLocalInstanceStatusesWithNoCache(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := SetClusterConfig()
	require.NoError(t, err)

	p, err := naming.ParsePath("test/svc/foo")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(p.ConfigFile()), 0700))
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte("[DEFAULT]\nnodes = "+hostname.Hostname()+"\n[fs#1]\ntype = flag\n"), 0600))
	m, err := localInstanceStatuses(p)
	require.NoError(t, err)
	require.Len(t, m, 1)
	for _, st := range m {
		assert.Contains(t, st.Resources, "fs#1")
	}

	m, err = localInstanceStatuses(naming.Path{Namespace: "test", Kind: naming.KindCfg, Name: "foo"})
	require.NoError(t, err)
	assert.Empty(t, m, "an object holding no resource")
}

// A required keyword left empty leaves the resource not configured, its
// status not known, which is not stopped, and the change setting it is not
// refused: the empty value designates nothing the resource could have set up.
func TestChangeFromEmptyIsNotRefused(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := SetClusterConfig()
	require.NoError(t, err)

	p, err := naming.ParsePath("test/svc/foo")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(p.ConfigFile()), 0700))
	broken := "[DEFAULT]\nnodes = " + hostname.Hostname() + "\n[disk#1]\ntype = loop\n"
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte(broken), 0600))
	m, err := localInstanceStatuses(p)
	require.NoError(t, err)
	st := m[hostname.Hostname()]
	require.NotNil(t, st)
	require.Contains(t, st.Resources, "disk#1")
	assert.Equal(t, status.Undef, st.Resources["disk#1"].Status)

	assert.NoError(t, CheckLocalUpdateStopToChange(p, nil, nil, []keyop.T{*keyop.Parse("disk#1.file=/srv/foo.img")}))
	assert.ErrorIs(t, CheckLocalUpdateStopToChange(p, []string{"disk#1"}, nil, nil), ErrStopToChange, "the removal of a resource not known to be stopped")
}
