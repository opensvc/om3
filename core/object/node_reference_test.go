package object_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/key"
)

// An object configuration references the node environment, labels and
// public keywords, but not a secret of the node: the uuid the node
// authenticates to the collector with would be handed to whoever reads the
// object configuration evaluated, or the files an install template renders
// from it.
func TestNodeSecretIsNotReferenced(t *testing.T) {
	testhelper.Setup(t)
	const fakeUUID = "11111111-2222-3333-4444-555555555555"
	nodeConf := "[node]\nuuid = " + fakeUUID + "\nsec_zone = dmz\n\n[env]\nx = hello\n"
	require.NoError(t, os.WriteFile(rawconfig.NodeConfigFile(), []byte(nodeConf), 0600))

	p, err := naming.ParsePath("ci/svc/ref")
	require.NoError(t, err)
	conf := "[DEFAULT]\nnodes = *\n\n[env]\nu = {node.uuid}\nv = {node.node.uuid}\na = {node.env.x}\nz = {node.sec_zone}\n"
	require.NoError(t, os.MkdirAll(filepath.Dir(p.ConfigFile()), 0755))
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte(conf), 0644))

	o, err := object.NewSvc(p)
	require.NoError(t, err)
	var i any = o
	c := i.(object.Configurer).Config()

	// A refused reference is left unevaluated, as any reference that does
	// not resolve.
	for option, ref := range map[string]string{"u": "{node.uuid}", "v": "{node.node.uuid}"} {
		v, _ := c.GetStringStrict(key.New("env", option))
		assert.NotContains(t, v, fakeUUID, "env.%s", option)
		assert.Equal(t, ref, v, "env.%s", option)
	}

	v, err := c.GetStringStrict(key.New("env", "a"))
	require.NoError(t, err)
	assert.Equal(t, "hello", v)

	v, err = c.GetStringStrict(key.New("env", "z"))
	require.NoError(t, err)
	assert.Equal(t, "dmz", v)
}
