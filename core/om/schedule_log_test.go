package om

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/hostname"
)

// The scheduler logs the runs of the jobs asking it, as a run asked by a
// user is: a task or a sync whose log keyword is true, false by default, and
// the compliance runs, of the objects and of the node. The other jobs are
// not.
func TestSchedulesLogEntries(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	require.NoError(t, os.WriteFile(
		filepath.Join(env.Root, "etc", "cluster.conf"),
		[]byte("[cluster]\nname = clu1\nnodes = "+hostname.Hostname()+"\n"),
		0644))

	p, err := naming.ParsePath("svc1")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(p.ConfigFile()), 0755))
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte(`[DEFAULT]
nodes = `+hostname.Hostname()+`

[task#logged]
type = host
command = echo TASK1
schedule = @2
log = true

[task#quiet]
type = host
command = echo TASK2
schedule = @2

[sync#logged]
type = zfssnap
dataset = tank/svc1
schedule = @10
log = true

[sync#quiet]
type = zfssnap
dataset = tank/svc1
schedule = @10
`), 0644))

	o, err := object.NewActor(p)
	require.NoError(t, err)
	logged := make(map[string]bool)
	for _, e := range o.Schedules() {
		logged[e.Key] = e.Log
	}
	for k, want := range map[string]bool{
		"task#logged.schedule": true,
		"task#quiet.schedule":  false,
		"sync#logged.schedule": true,
		"sync#quiet.schedule":  false,
		"comp_schedule":        true,
		"status_schedule":      false,
	} {
		require.Contains(t, logged, k)
		assert.Equalf(t, want, logged[k], "%s", k)
	}

	n, err := object.NewNode()
	require.NoError(t, err)
	for _, e := range n.Schedules() {
		assert.Equalf(t, e.Action == "compliance_auto", e.Log, "node job %s", e.Action)
	}
}
