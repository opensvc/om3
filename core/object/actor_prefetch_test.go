package object_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/freeze"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/hostname"

	_ "github.com/opensvc/om3/v3/drivers/rescontainerdocker"
	_ "github.com/opensvc/om3/v3/drivers/resfsflag"
)

// fakeDocker puts a docker in the PATH whose store holds the images named
// in the "images" file of the returned directory, one per line, pulls the
// ones whose name has no "missing", and logs its pulls in "pulls".
func fakeDocker(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	script := `#!/bin/sh
dir=` + dir + `
case "$1 $2" in
"image inspect")
	if grep -qxF "$5" $dir/images 2>/dev/null; then echo sha256:abc; exit 0; fi
	echo "Error response from daemon: No such image: $5" >&2; exit 1;;
"image pull")
	echo "$3" >>$dir/pulls
	case "$3" in *missing*) echo "Error response from daemon: manifest unknown" >&2; exit 1;; esac
	echo "$3" >>$dir/images;;
esac
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0o755))
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	return dir
}

// The prefetch pulls the images of the container resources of the instance
// the store does not hold, and reports each: a frozen instance is
// prefetched as another, a disabled resource is not, and a resource with no
// image is none of its business.
func TestPrefetch(t *testing.T) {
	env := testhelper.Setup(t)
	dir := fakeDocker(t)
	nodesInfo := fmt.Sprintf(`{%q: {"labels": {}}}`, hostname.Hostname())
	require.NoError(t, os.WriteFile(filepath.Join(env.Root, "var", "nodes_info.json"), []byte(nodesInfo), 0o644))
	cluster.ConfigData.Set(&cluster.Config{Name: "cluster1", Nodes: []string{hostname.Hostname()}})
	p, err := naming.ParsePath("test/svc/prefetch")
	require.NoError(t, err)
	o, err := object.NewSvc(p, object.WithConfigData([]byte(fmt.Sprintf(`[DEFAULT]
nodes = %s

[fs#1]
type = flag

[container#1]
type = docker
image = registry.example.com/app:1

[container#2]
type = docker
image = registry.example.com/missing:1

[container#3]
type = docker
image = registry.example.com/disabled:1
disable = true
`, hostname.Hostname()))))
	require.NoError(t, err)
	require.NoError(t, freeze.Freeze(p.FrozenFile()))

	pulls := func() string {
		b, _ := os.ReadFile(filepath.Join(dir, "pulls"))
		return string(b)
	}
	states := func(reports []resource.PrefetchReport) map[string]string {
		m := make(map[string]string)
		for _, report := range reports {
			m[report.RID] = report.State
		}
		return m
	}

	reports, err := o.Prefetch(context.Background(), "")
	require.NoError(t, err, "a pull that failed is an outcome, not an error")
	assert.Equal(t, map[string]string{
		"container#1": resource.PrefetchPulled,
		"container#2": resource.PrefetchFailed,
	}, states(reports))
	assert.Equal(t, "registry.example.com/app:1\nregistry.example.com/missing:1\n", pulls())
	outcome, ok := resource.ReadPrefetchOutcome(resource.Pull{Engine: "docker", Image: "registry.example.com/missing:1"})
	require.True(t, ok, "the failure is recorded for the status")
	assert.Equal(t, resource.PrefetchFailed, outcome.State)
	assert.Contains(t, outcome.Error, "manifest unknown")

	// An image the store holds is not pulled again, and a resource
	// selector narrows the prefetch.
	reports, err = o.Prefetch(context.Background(), "container#1")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"container#1": resource.PrefetchPresent}, states(reports))
	assert.Equal(t, "registry.example.com/app:1\nregistry.example.com/missing:1\n", pulls())
}
