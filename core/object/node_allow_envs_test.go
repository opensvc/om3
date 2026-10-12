package object

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/testhelper"
)

// A node naming no env allows every env but on a PRD node, which allows the
// PRD objects only. A node naming envs allows those alone.
func TestANodeAllowsTheEnvsItNamesOrElseItsEnvDecides(t *testing.T) {
	for _, tc := range []struct {
		env, nodeEnv string
		allowEnvs    []string
		want         bool
	}{
		{"PRD", "PRD", nil, true},
		{"PPRD", "PRD", nil, false},
		{"TST", "PRD", nil, false},
		{"PRD", "DRP", nil, true},
		{"TST", "DEV", nil, true},
		{"PPRD", "PRD", []string{"PRD", "PPRD"}, true},
		{"PRD", "PRD", []string{"PRD", "PPRD"}, true},
		{"TST", "PRD", []string{"PRD", "PPRD"}, false},
		{"TST", "DEV", []string{"DEV"}, false},
	} {
		assert.Equal(t, tc.want, allowsEnv(tc.env, tc.nodeEnv, tc.allowEnvs), "%s object on a %s node allowing %v", tc.env, tc.nodeEnv, tc.allowEnvs)
	}
}

// The node env and the envs it allows are the node keywords of the node
// configuration, or of the cluster's, the node's winning.
func TestTheNodeEnvIsReadFromTheNodeOrClusterConfiguration(t *testing.T) {
	testhelper.Setup(t)
	write := func(p, s string) {
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0755))
		require.NoError(t, os.WriteFile(p, []byte(s), 0644))
	}
	write(rawconfig.ClusterConfigFile(), "[cluster]\nname = c1\n\n[node]\nenv = PRD\nallow_envs = PRD PPRD\n")
	n, err := NewNode()
	require.NoError(t, err)
	assert.Equal(t, "PRD", n.Env(), "set in the cluster configuration")
	assert.Equal(t, []string{"PRD", "PPRD"}, n.AllowEnvs())
	assert.True(t, n.AllowsEnv("PPRD"))
	assert.False(t, n.AllowsEnv("TST"))

	write(rawconfig.NodeConfigFile(), "[node]\nenv = DEV\n")
	n, err = NewNode()
	require.NoError(t, err)
	assert.Equal(t, "DEV", n.Env(), "the node configuration wins")
}

// An object naming no env takes the env of its node.
func TestAnObjectNamingNoEnvTakesTheNodeEnv(t *testing.T) {
	testhelper.Setup(t)
	p := rawconfig.ClusterConfigFile()
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0755))
	require.NoError(t, os.WriteFile(p, []byte("[cluster]\nname = c1\n\n[node]\nenv = PRD\n"), 0644))
	path, err := naming.ParsePath("test/svc/noenv")
	require.NoError(t, err)
	o, err := New(path, WithConfigData([]byte("[DEFAULT]\nnodes = *\n")), WithVolatile(true))
	require.NoError(t, err)
	assert.Equal(t, "PRD", o.(*svc).Env())
}
