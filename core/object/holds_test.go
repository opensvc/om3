package object_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/resource"

	_ "github.com/opensvc/om3/v3/drivers/rescontainerdocker"
	_ "github.com/opensvc/om3/v3/drivers/resdisklv"
	_ "github.com/opensvc/om3/v3/drivers/resfsdir"
	_ "github.com/opensvc/om3/v3/drivers/resfshost"
	_ "github.com/opensvc/om3/v3/drivers/resiphost"
)

func holdsOf(t *testing.T, p naming.Path, conf string, nodes ...string) []resource.ObjectHold {
	t.Helper()
	oc, err := object.NewConfigurer(p, object.WithConfigData([]byte(conf)), object.WithVolatile(true), object.WithStaticReferences())
	require.NoError(t, err)
	return object.Holds(p, oc, nodes)
}

// A value is held on the nodes it evaluates to it on, or on every cluster
// node for a shared resource, whatever the driver.
func TestHoldsScope(t *testing.T) {
	p, err := naming.ParsePath("ns1/svc/web")
	require.NoError(t, err)
	nodes := []string{"n1", "n2", "n3"}
	mnt := func(l []resource.ObjectHold) map[string]resource.ObjectHold {
		m := make(map[string]resource.ObjectHold)
		for _, h := range l {
			if h.Class == resource.HoldMountPoint {
				m[h.Value] = h
			}
		}
		return m
	}

	m := mnt(holdsOf(t, p, "[DEFAULT]\nnodes = n1 n2 n3\n[fs#1]\ntype = ext4\ndev = /dev/sdb\nmnt@n1 = /a\nmnt@n2 = /a\nmnt@n3 = /b\nshared = false\n", nodes...))
	assert.Equal(t, []string{"n1", "n2"}, m["/a"].Nodes)
	assert.Equal(t, []string{"n3"}, m["/b"].Nodes)
	assert.False(t, m["/a"].Cluster)

	m = mnt(holdsOf(t, p, "[DEFAULT]\nnodes = n1 n2 n3\n[fs#1]\ntype = ext4\ndev = /dev/sdb\nmnt@n1 = /a\nmnt@n2 = /a\nmnt@n3 = /b\nshared = true\n", nodes...))
	assert.True(t, m["/a"].Cluster)
	assert.True(t, m["/b"].Cluster)

	m = mnt(holdsOf(t, p, "[DEFAULT]\nnodes = n1 n2 n3\n[fs#1]\ntype = ext4\ndev = /dev/sdb\nmnt = /a\nshared = false\n", nodes...))
	assert.Equal(t, []string{"n1", "n2", "n3"}, m["/a"].Nodes)
}

// The holds of drivers meet across drivers, a value referencing what a
// resource exposes is held by that resource alone, and a disabled resource
// holds nothing.
func TestHoldsDrivers(t *testing.T) {
	p, err := naming.ParsePath("ns1/svc/web")
	require.NoError(t, err)
	l := holdsOf(t, p, `[DEFAULT]
nodes = n1
[disk#1]
type = lv
vg = datavg
name = web
[fs#1]
type = ext4
dev = {disk#1.exposed_devs[0]}
mnt = /srv/{name}
[fs#2]
type = ext4
dev = /dev/sdz
mnt = /srv/off
disable = true
[container#1]
type = docker
image = busybox
[ip#1]
type = host
name = 10.0.0.5
`, "n1")
	got := make([]string, 0)
	for _, h := range l {
		got = append(got, h.RID+" "+h.Class+" "+h.Value)
	}
	assert.ElementsMatch(t, []string{
		"container#1 container name ns1..web.container.1",
		"disk#1 block device /dev/datavg/web",
		"disk#1 logical volume datavg/web",
		"fs#1 mount point /srv/web",
		"ip#1 ip address 10.0.0.5",
	}, got)

	other := holdsOf(t, naming.Path{Namespace: "ns2", Kind: naming.KindSvc, Name: "db"}, "[DEFAULT]\nnodes = n1\n[fs#1]\ntype = ext4\ndev = /dev/datavg/web\nmnt = /srv/db\n", "n1")
	conflicts := 0
	for _, a := range l {
		for _, b := range other {
			if a.Overlaps(b) {
				conflicts++
				assert.Equal(t, "/dev/datavg/web", a.Value)
			}
		}
	}
	assert.Equal(t, 1, conflicts, "a device path meets the lv exposing it")
}

// A directory is held as a mount point is, unless it is in a volume, which
// holds it.
func TestHoldsDirectory(t *testing.T) {
	p := naming.Path{Namespace: "ns1", Kind: naming.KindSvc, Name: "web"}
	l := holdsOf(t, p, "[DEFAULT]\nnodes = n1\n[fs#1]\ntype = directory\npath = /srv/{name}/logs/\n[fs#2]\ntype = directory\npath = volume#1:/conf\n[fs#3]\ntype = directory\npath = data/conf\n", "n1")
	got := make([]string, 0)
	for _, h := range l {
		got = append(got, h.RID+" "+h.Class+" "+h.Value)
	}
	assert.Equal(t, []string{"fs#1 mount point /srv/web/logs"}, got)
}
