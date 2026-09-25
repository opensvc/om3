//go:build linux

package object_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/claim"
	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/testhelper"

	_ "github.com/opensvc/om3/v3/core/driverdb"
)

const nsClaims = `[claim#1]
type = cpu
limit = 800%
default = 25%

[claim#2]
type = memory
limit = 4g
default = 128m
`

func computeClaims(t *testing.T, namespace, nscfg, config string) map[string]int64 {
	t.Helper()
	env := testhelper.Setup(t)
	nodesInfo := `{"n1": {"labels": {}}, "n2": {"labels": {}}, "n3": {"labels": {}}}`
	require.NoError(t, os.WriteFile(filepath.Join(env.Root, "var", "nodes_info.json"), []byte(nodesInfo), 0644))
	// The nodes an object names are read against the cluster nodes, which
	// another test of the package may have set.
	previous := cluster.ConfigData.Get()
	clusterConfig := cluster.Config{Name: "cluster1", Nodes: []string{"n1", "n2", "n3"}}
	cluster.ConfigData.Set(&clusterConfig)
	t.Cleanup(func() { cluster.ConfigData.Set(previous) })
	if nscfg != "" {
		p := naming.Path{Namespace: namespace, Kind: naming.KindNscfg, Name: "namespace"}
		require.NoError(t, os.MkdirAll(filepath.Dir(p.ConfigFile()), 0755))
		require.NoError(t, os.WriteFile(p.ConfigFile(), []byte(nscfg), 0644))
	}
	p := naming.Path{Namespace: namespace, Kind: naming.KindSvc, Name: "web"}
	o, err := object.NewSvc(p, object.WithConfigData([]byte(config)))
	require.NoError(t, err)
	var i any = o
	claimer, ok := i.(object.ComputeClaimer)
	require.True(t, ok)
	m, err := claimer.ComputeClaims()
	require.NoError(t, err)
	return m
}

// A container capped by nothing is counted for the default of its namespace.
func TestAFailoverClaimsOneInstanceWithTheDefaults(t *testing.T) {
	m := computeClaims(t, "ci", nsClaims, `[DEFAULT]
nodes = n1 n2

[container#1]
type = podman
image = nginx
pg_cpu_quota = 50%
pg_mem_limit = 256m

[container#2]
type = podman
image = nginx
`)
	assert.Equal(t, int64(750), m[claim.TypeCPU])
	assert.Equal(t, int64(384*1024*1024), m[claim.TypeMemory])
}

func TestAFlexClaimsItsTargetInstances(t *testing.T) {
	m := computeClaims(t, "ci", nsClaims, `[DEFAULT]
nodes = n1 n2 n3
topology = flex
flex_target = 2

[container#1]
type = podman
image = nginx
pg_cpu_quota = 50%
`)
	assert.Equal(t, int64(1000), m[claim.TypeCPU])
	assert.Equal(t, int64(256*1024*1024), m[claim.TypeMemory])
}

// The instances not started run their standby resources.
func TestTheStandbyResourcesAreClaimedOnEveryNode(t *testing.T) {
	m := computeClaims(t, "ci", nsClaims, `[DEFAULT]
nodes = n1 n2

[container#1]
type = podman
image = nginx
standby = true
pg_cpu_quota = 50%

[container#2]
type = podman
image = nginx
`)
	assert.Equal(t, int64(750+500), m[claim.TypeCPU])
}

// Any node may run the started instance, so it is counted where it takes the
// most.
func TestTheStartedInstanceIsCountedWhereItTakesTheMost(t *testing.T) {
	m := computeClaims(t, "ci", nsClaims, `[DEFAULT]
nodes = n1 n2

[container#1]
type = podman
image = nginx
pg_cpu_quota = 50%
pg_cpu_quota@n2 = 200%
`)
	assert.Equal(t, int64(2000), m[claim.TypeCPU])
}

// An app process is not given the default, and is capped by its object or by
// nothing.
func TestAnUncappedAppIsUnboundedUnlessTheObjectIsCapped(t *testing.T) {
	config := `[DEFAULT]
nodes = n1 n2

[app#1]
type = simple
start = /bin/true
`
	m := computeClaims(t, "ci", nsClaims, config)
	assert.Equal(t, claim.Unbounded, m[claim.TypeCPU])

	m = computeClaims(t, "ci", nsClaims, `[DEFAULT]
nodes = n1 n2
pg_cpu_quota = 100%@2

[app#1]
type = simple
start = /bin/true
`)
	assert.Equal(t, int64(2000), m[claim.TypeCPU])
	assert.Equal(t, claim.Unbounded, m[claim.TypeMemory])
}

// A container under a capped object is not given the default, which would
// cap it below what the object admits.
func TestTheDefaultIsNotGivenUnderACappedObject(t *testing.T) {
	m := computeClaims(t, "ci", nsClaims, `[DEFAULT]
nodes = n1 n2
pg_cpu_quota = 300%

[container#1]
type = podman
image = nginx

[container#2]
type = podman
image = nginx
`)
	assert.Equal(t, int64(3000), m[claim.TypeCPU], "the object cap bounds what nothing else caps")
}
