package object_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/hostname"

	_ "github.com/opensvc/om3/v3/core/driverdb"
)

// pullsOf returns the images the node prefetches for an object of the
// configuration conf, on a node having both engines, with the node and
// cluster configurations given.
func pullsOf(t *testing.T, nodeConf, clusterConf, conf string) (map[string]resource.Pull, []string) {
	t.Helper()
	testhelper.Setup(t)
	require.NoError(t, os.MkdirAll(rawconfig.Paths.Etc, 0o755))
	require.NoError(t, os.WriteFile(rawconfig.NodeConfigFile(), []byte(nodeConf), 0o600))
	require.NoError(t, os.WriteFile(rawconfig.ClusterConfigFile(), []byte(clusterConf), 0o600))
	caps := []string{
		driver.NewID(driver.GroupContainer, "docker").Cap(),
		driver.NewID(driver.GroupContainer, "podman").Cap(),
		driver.NewID(driver.GroupContainer, "oci").Cap(),
	}
	capabilities.Register(func(context.Context) ([]string, error) { return caps, nil })
	require.NoError(t, capabilities.Scan(t.Context()))
	p, err := naming.ParsePath("ns1/svc/web")
	require.NoError(t, err)
	oc, err := object.NewConfigurer(p, object.WithVolatile(true), object.WithConfigData([]byte(conf)))
	require.NoError(t, err)
	n, err := object.NewNode(object.WithVolatile(true))
	require.NoError(t, err)
	return object.Pulls(p, oc, n)
}

// The image a resource prefetches is pulled by the engine its start runs
// with: the one of its type, and the one of the node for a type unset or
// oci. A task, a kvm container, a disabled or an encap resource prefetch
// nothing.
func TestPullsEngines(t *testing.T) {
	conf := `[DEFAULT]
nodes = ` + hostname.Hostname() + `

[container#unset]
image = nginx

[container#oci]
type = oci
image = nginx:1

[container#docker]
type = docker
image = ghcr.io/opensvc/pause

[container#podman]
type = podman
image = ghcr.io/opensvc/pause

[container#rootless]
type = podman
image = ghcr.io/opensvc/pause
rootless_user = app

[container#creds]
type = docker
image = registry.example.com/app:1
registry_creds = creds

[container#disabled]
type = docker
image = registry.example.com/disabled:1
disable = true

[container#encap]
type = docker
image = registry.example.com/encap:1
encap = true

[container#kvm]
type = kvm
name = vm1

[task#1]
type = docker
image = registry.example.com/task:1
command = /bin/true

[fs#1]
type = flag
`
	pulls, unevaluated := pullsOf(t, "[node]\noci = docker\n", "[cluster]\nname = cluster1\n", conf)
	assert.Empty(t, unevaluated)
	assert.Equal(t, map[string]resource.Pull{
		"container#unset":    {Engine: "docker", Image: "docker.io/library/nginx:latest"},
		"container#oci":      {Engine: "docker", Image: "docker.io/library/nginx:1"},
		"container#docker":   {Engine: "docker", Image: "ghcr.io/opensvc/pause:latest"},
		"container#podman":   {Engine: "podman", Image: "ghcr.io/opensvc/pause:latest"},
		"container#rootless": {Engine: "podman", Store: "app", Image: "ghcr.io/opensvc/pause:latest"},
		"container#creds":    {Engine: "docker", Image: "registry.example.com/app:1", Creds: "ns1/sec/creds"},
	}, pulls)

	pulls, _ = pullsOf(t, "[node]\noci = podman\n", "[cluster]\nname = cluster1\n", conf)
	assert.Equal(t, "podman", pulls["container#unset"].Engine, "the engine of the node")
	assert.Equal(t, "podman", pulls["container#oci"].Engine, "the engine of the node")
	assert.Equal(t, "docker", pulls["container#docker"].Engine)
}

// A resource is prefetched as its image_prefetch keyword says, and as the
// node says when it says nothing: the node configuration first, then the
// cluster one, then true.
func TestPullsEnabled(t *testing.T) {
	conf := `[DEFAULT]
nodes = ` + hostname.Hostname() + `

[container#unset]
type = docker
image = nginx

[container#on]
type = docker
image = nginx
image_prefetch = true

[container#off]
type = docker
image = nginx
image_prefetch = false

[container#scoped]
type = docker
image = nginx
image_prefetch = true
image_prefetch@` + hostname.Hostname() + ` = false
`
	rids := func(pulls map[string]resource.Pull) []string {
		l := make([]string, 0)
		for _, rid := range []string{"container#unset", "container#on", "container#off", "container#scoped"} {
			if _, ok := pulls[rid]; ok {
				l = append(l, rid)
			}
		}
		return l
	}
	for name, tc := range map[string]struct {
		node     string
		cluster  string
		expected []string
	}{
		"no default set": {
			node:     "",
			cluster:  "[cluster]\nname = cluster1\n",
			expected: []string{"container#unset", "container#on"},
		},
		"disabled by the cluster": {
			node:     "",
			cluster:  "[cluster]\nname = cluster1\n[node]\nimage_prefetch = false\n",
			expected: []string{"container#on"},
		},
		"disabled by the node": {
			node:     "[node]\nimage_prefetch = false\n",
			cluster:  "[cluster]\nname = cluster1\n",
			expected: []string{"container#on"},
		},
		"disabled by the cluster, enabled by the node": {
			node:     "[node]\nimage_prefetch = true\n",
			cluster:  "[cluster]\nname = cluster1\n[node]\nimage_prefetch = false\n",
			expected: []string{"container#unset", "container#on"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			pulls, _ := pullsOf(t, tc.node, tc.cluster, conf)
			assert.Equal(t, tc.expected, rids(pulls))
		})
	}
}

// The image is the one this node evaluates, references and scopes applied,
// and a resource whose image evaluates to no image, as to an option of the
// engine, is reported, with no Pull.
func TestPullsEvaluateTheImageAsTheNode(t *testing.T) {
	conf := `[DEFAULT]
nodes = ` + hostname.Hostname() + ` other

[env]
tag = 1.27
option = --privileged

[container#ref]
type = docker
image = nginx:{env.tag}

[container#scoped]
type = docker
image = nginx:1
image@` + hostname.Hostname() + ` = nginx:2
image@other = nginx:3

[container#broken]
type = docker
image = {env.option}

[container#default]
type = docker
`
	pulls, unevaluated := pullsOf(t, "", "[cluster]\nname = cluster1\n", conf)
	assert.Equal(t, []string{"container#broken"}, unevaluated)
	assert.Equal(t, map[string]resource.Pull{
		"container#ref":     {Engine: "docker", Image: "docker.io/library/nginx:1.27"},
		"container#scoped":  {Engine: "docker", Image: "docker.io/library/nginx:2"},
		"container#default": {Engine: "docker", Image: "ghcr.io/opensvc/pause:latest"},
	}, pulls)
}

// A disabled instance starts nothing, and prefetches nothing.
func TestPullsOfADisabledInstance(t *testing.T) {
	pulls, unevaluated := pullsOf(t, "", "[cluster]\nname = cluster1\n", "[DEFAULT]\ndisable = true\n[container#1]\ntype = docker\nimage = nginx\n")
	assert.Empty(t, pulls)
	assert.Empty(t, unevaluated)
}
