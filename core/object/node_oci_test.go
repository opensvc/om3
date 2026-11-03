package object

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/testhelper"
)

// The node.oci keyword names the engine of the oci resources. A value no
// engine has is ignored for the engine the node has, docker with none, as in
// v2.
func TestOCIEngine(t *testing.T) {
	testhelper.Setup(t)
	for conf, want := range map[string]string{
		"[node]\noci = podman\n": "podman",
		"[node]\noci = docker\n": "docker",
		"[node]\noci = lxc\n":    "docker",
		"[node]\n":               "docker",
	} {
		n, err := NewNode(WithVolatile(true), WithConfigData([]byte(conf)))
		require.NoError(t, err)
		assert.Equal(t, want, n.OCIEngine(), conf)
	}
}
