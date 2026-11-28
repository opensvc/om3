package object_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/freeze"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/hostname"

	_ "github.com/opensvc/om3/v3/drivers/rescontainerdocker"
	_ "github.com/opensvc/om3/v3/drivers/resfsflag"
)

// fakeDocker puts a docker in the PATH whose store holds the images named
// in the "images" file of the returned directory, one per line, pulls the
// ones whose name has no "missing", logs its pulls in "pulls", and loads an
// archive as the names of the images it brings.
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
"image load")
	cat >>$dir/images;;
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

// fakeDaemon answers the prefetch in the place of the daemon of the node:
// where the resources are up, and the archive of the images the nodes hold,
// which is the name of the image. It records the nodes asked for an image,
// and what they were asked of their docker store, the one of the resources
// of the tests.
type fakeDaemon struct {
	up    map[string][]string
	holds map[string][]string
	asked []string
	nodes []string
}

func (t *fakeDaemon) GetResourcesWithResponse(_ context.Context, params *api.GetResourcesParams, _ ...api.RequestEditorFn) (*api.GetResourcesResponse, error) {
	items := make(api.ResourceItems, 0)
	for rid, nodes := range t.up {
		for _, node := range nodes {
			items = append(items, api.ResourceItem{
				Meta: api.ResourceMeta{Node: node, Object: *params.Path, RID: rid},
				Data: api.Resource{Status: &resource.Status{Status: status.Up}},
			})
		}
	}
	return &api.GetResourcesResponse{JSON200: &api.ResourceList{Items: items}}, nil
}

func (t *fakeDaemon) GetNodeImageSave(_ context.Context, nodename api.InPathNodeName, params *api.GetNodeImageSaveParams, _ ...api.RequestEditorFn) (*http.Response, error) {
	t.nodes = append(t.nodes, nodename)
	if params.Engine == "docker" {
		t.asked = append(t.asked, nodename+" "+params.Ref)
	}
	resp := &http.Response{StatusCode: http.StatusNotFound, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("{}"))}
	for _, image := range t.holds[nodename] {
		if image == params.Ref && params.Engine == "docker" && params.Store == nil {
			resp.StatusCode = http.StatusOK
			resp.Header.Set(api.HeaderImageID, "sha256:abc")
			resp.Header.Set(api.HeaderImageIDKind, resource.ImageIDConfig)
			resp.Header.Set(api.HeaderPlatform, resource.NodePlatform())
			resp.Body = io.NopCloser(strings.NewReader(image + "\n"))
		}
	}
	return resp, nil
}

// The nodes where the resource is up are asked first, then the others, in
// the order of the nodes keyword.
func TestOrderImageSourceNodes(t *testing.T) {
	nodes := []string{"n2", "n3", "n4", "n5"}
	assert.Equal(t, nodes, object.OrderImageSourceNodes(nodes, nil))
	assert.Equal(t, []string{"n3", "n5", "n2", "n4"}, object.OrderImageSourceNodes(nodes, map[string]bool{"n5": true, "n3": true, "n9": true}),
		"a node that is not one of the object is never asked, up or not")
}

// A missing image is copied from a node of the object holding it, the one
// running the container first, and pulled from its registry when no node
// holds it. The nodes asked are the ones of the nodes keyword, and no
// other: not a node of the cluster the object does not name, though it
// holds the image, nor this one.
func TestPrefetchCopiesFromTheNodesOfTheObject(t *testing.T) {
	env := testhelper.Setup(t)
	dir := fakeDocker(t)
	localhost := hostname.Hostname()
	nodesInfo := fmt.Sprintf(`{%q: {"labels": {}}, "n2": {"labels": {}}, "n3": {"labels": {}}, "n4": {"labels": {}}}`, localhost)
	require.NoError(t, os.WriteFile(filepath.Join(env.Root, "var", "nodes_info.json"), []byte(nodesInfo), 0o644))
	cluster.ConfigData.Set(&cluster.Config{Name: "cluster1", Nodes: []string{localhost, "n2", "n3", "n4"}})
	daemon := &fakeDaemon{
		up: map[string][]string{"container#1": {"n3"}, "container#2": {"n4"}},
		holds: map[string][]string{
			"n2": {"registry.example.com/app:1"},
			"n3": {"registry.example.com/app:1"},
			"n4": {"registry.example.com/app:1", "registry.example.com/elsewhere:1", "registry.example.com/missing:1"},
		},
	}
	defer object.SetImageSourceClient(daemon)()
	p, err := naming.ParsePath("test/svc/peercopy")
	require.NoError(t, err)
	o, err := object.NewSvc(p, object.WithConfigData([]byte(fmt.Sprintf(`[DEFAULT]
nodes = n2 %s n3

[container#1]
type = docker
image = registry.example.com/app:1

[container#2]
type = docker
image = registry.example.com/elsewhere:1

[container#3]
type = docker
image = registry.example.com/missing:1
`, localhost))))
	require.NoError(t, err)

	reports, err := o.Prefetch(context.Background(), "")
	require.NoError(t, err)
	byRID := make(map[string]resource.PrefetchReport)
	for _, report := range reports {
		byRID[report.RID] = report
	}
	require.Len(t, byRID, 3)

	assert.Equal(t, resource.PrefetchPulled, byRID["container#1"].State)
	assert.Equal(t, "peer n3", byRID["container#1"].Source, "the node running the container is asked first")
	assert.Equal(t, "sha256:abc", byRID["container#1"].ImageID)

	assert.Equal(t, resource.PrefetchPulled, byRID["container#2"].State)
	assert.Equal(t, resource.PrefetchSourceRegistry, byRID["container#2"].Source, "n4 holds the image, and is not a node of the object")

	assert.Equal(t, resource.PrefetchFailed, byRID["container#3"].State)
	assert.Contains(t, byRID["container#3"].Error, "peers: n2: image not found, n3: image not found; registry: ")
	assert.Contains(t, byRID["container#3"].Error, "manifest unknown")

	assert.Equal(t, []string{
		"n3 registry.example.com/app:1",
		"n2 registry.example.com/elsewhere:1",
		"n3 registry.example.com/elsewhere:1",
		"n2 registry.example.com/missing:1",
		"n3 registry.example.com/missing:1",
	}, daemon.asked)
	assert.NotContains(t, daemon.nodes, "n4")
	assert.NotContains(t, daemon.nodes, localhost)
	b, _ := os.ReadFile(filepath.Join(dir, "pulls"))
	assert.Equal(t, "registry.example.com/elsewhere:1\nregistry.example.com/missing:1\n", string(b), "an image copied is not pulled")

	outcome, ok := resource.ReadPrefetchOutcome(resource.Pull{Engine: "docker", Image: "registry.example.com/app:1"})
	require.True(t, ok)
	assert.Equal(t, "peer n3", outcome.Source, "the outcome recorded tells the source")
}
