package object_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/hostname"

	_ "github.com/opensvc/om3/v3/drivers/resfshost"
	_ "github.com/opensvc/om3/v3/drivers/resvol"
)

type triggerCommander interface {
	TriggerCommand(rid, hook string) (string, error)
}

func newTriggerTestSvc(t *testing.T) triggerCommander {
	t.Helper()
	env := testhelper.Setup(t)
	nodesInfo := fmt.Sprintf(`{%q: {"labels": {}}}`, hostname.Hostname())
	require.NoError(t, os.WriteFile(filepath.Join(env.Root, "var", "nodes_info.json"), []byte(nodesInfo), 0o644))
	cluster.ConfigData.Set(&cluster.Config{Name: "cluster1", Nodes: []string{hostname.Hostname()}})
	p, err := naming.ParsePath("test/svc/c")
	require.NoError(t, err)
	o, err := object.NewSvc(p, object.WithConfigData([]byte(fmt.Sprintf(`[DEFAULT]
nodes = %s

[volume#1]
name = {name}-vol
size = 10m
post_provision = /bin/sh -c "echo mnt=[{volume#1.mnt}] > /tmp/out"
blocking_post_provision = /bin/echo {volume#1.mnt}/cache
pre_provision = /bin/echo {name} {volume#1.name}
post_start = awk '{print $1}' /etc/hosts
`, hostname.Hostname()))))
	require.NoError(t, err)
	o.ConfigureResources()
	return o
}

// A trigger naming the mount point of a volume not provisioned yet is not
// evaluated with the mount point empty: "rm -rf {volume#1.mnt}/cache" would
// remove /cache. It used to be, as the post_provision of the volume ran with
// the command evaluated before the volume was provisioned.
func TestATriggerNamingAVolumeNotProvisionedIsRefused(t *testing.T) {
	o := newTriggerTestSvc(t)

	for _, hook := range []string{"post_provision", "blocking_post_provision"} {
		_, err := o.TriggerCommand("volume#1", hook)
		require.ErrorIs(t, err, xconfig.ErrRefNotAvailable, hook)
		assert.ErrorContains(t, err, "{volume#1.mnt} evaluates empty", hook)
	}
}

// A trigger naming nothing evaluated at run time, or braces that are not a
// reference, is evaluated as before.
func TestATriggerNamingNoRuntimeReferenceIsEvaluated(t *testing.T) {
	o := newTriggerTestSvc(t)

	s, err := o.TriggerCommand("volume#1", "pre_provision")
	require.NoError(t, err)
	assert.Equal(t, "/bin/echo c c-vol", s)

	s, err = o.TriggerCommand("volume#1", "post_start")
	require.NoError(t, err)
	assert.Equal(t, "awk '{print $1}' /etc/hosts", s)

	s, err = o.TriggerCommand("volume#1", "pre_stop")
	require.NoError(t, err)
	assert.Empty(t, s, "a trigger not set")
}

// A trigger naming the mount point of a volume is evaluated at the time it
// runs, reading what the provision made.
func TestATriggerNamingAVolumeProvisionedReadsItsMountPoint(t *testing.T) {
	o := newTriggerTestSvc(t)

	// The volume, as its provision makes it.
	vp, err := naming.ParsePath("test/vol/c-vol")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(vp.ConfigFile()), 0o755))
	require.NoError(t, os.WriteFile(vp.ConfigFile(), []byte(fmt.Sprintf("[DEFAULT]\nnodes = %s\n[fs#1]\ntype = ext4\ndev = /dev/null\nmnt = /srv/c-vol\n", hostname.Hostname())), 0o644))

	s, err := o.TriggerCommand("volume#1", "post_provision")
	require.NoError(t, err)
	assert.Equal(t, `/bin/sh -c "echo mnt=[/srv/c-vol] > /tmp/out"`, s)

	s, err = o.TriggerCommand("volume#1", "blocking_post_provision")
	require.NoError(t, err)
	assert.Equal(t, "/bin/echo /srv/c-vol/cache", s)
}
