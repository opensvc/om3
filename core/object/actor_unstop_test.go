package object

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/flagfile"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/hostname"

	_ "github.com/opensvc/om3/v3/drivers/resfsflag"
)

// Unstop lowers the flags a stop raised and starts nothing: with a resource
// selector the flags of the resources selected, the instance flag staying,
// and without one every flag.
func TestUnstop(t *testing.T) {
	env := testhelper.Setup(t)
	nodesInfo := fmt.Sprintf(`{%q: {"labels": {}}}`, hostname.Hostname())
	require.NoError(t, os.WriteFile(filepath.Join(env.Root, "var", "nodes_info.json"), []byte(nodesInfo), 0o644))
	cluster.ConfigData.Set(&cluster.Config{Name: "cluster1", Nodes: []string{hostname.Hostname()}})
	p, err := naming.ParsePath("test/svc/unstop")
	require.NoError(t, err)
	o, err := NewSvc(p, WithConfigData([]byte(fmt.Sprintf(`[DEFAULT]
nodes = %s
orchestrate = ha

[fs#1]
type = flag

[fs#2]
type = flag
`, hostname.Hostname()))))
	require.NoError(t, err)
	a := o
	a.ConfigureResources()
	stopped := func(rid string) bool {
		v, err := resource.IsStopped(a.ResourceByID(rid))
		require.NoError(t, err)
		return v
	}
	require.NoError(t, a.SetStopped())
	for _, rid := range []string{"fs#1", "fs#2"} {
		f := filepath.Join(a.ResourceByID(rid).VarDir(), "stopped")
		require.NoError(t, os.MkdirAll(filepath.Dir(f), 0o755))
		require.NoError(t, os.WriteFile(f, nil, 0o644))
	}

	ctx := actioncontext.WithRID(context.Background(), "fs#1")
	require.NoError(t, a.Unstop(ctx))
	assert.False(t, stopped("fs#1"), "fs#1 selected")
	assert.True(t, stopped("fs#2"), "fs#2 not selected")
	assert.False(t, flagfile.At(p.StoppedFile()).IsZero(), "the instance flag stays on a resource selection")

	require.NoError(t, a.Unstop(context.Background()))
	assert.False(t, stopped("fs#2"))
	assert.True(t, flagfile.At(p.StoppedFile()).IsZero(), "the instance flag is lowered")
	assert.Equal(t, "down", a.ResourceByID("fs#1").Status(context.Background()).String(), "nothing started")
}
