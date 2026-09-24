//go:build linux

package object_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/key"

	_ "github.com/opensvc/om3/v3/core/driverdb"
)

// Building the resources of an object reads node keywords, as the prkey
// default, {node.node.prkey}, of a filesystem. That read made an empty [node]
// section in the configuration of the object, which the next write committed:
// a resize recording the size it reached left one in every volume it grew.
func TestBuildingTheResourcesLeavesNoNodeSectionInTheConfiguration(t *testing.T) {
	testhelper.Setup(t)
	p, err := naming.ParsePath("ci/vol/alpha")
	require.NoError(t, err)
	config := "[DEFAULT]\nnodes = *\nsize = 1mi\n\n[fs#0]\ntype = tmpfs\ndev = none\nmnt = /srv/alpha\nsize = {DEFAULT.size}\n"
	require.NoError(t, os.MkdirAll(filepath.Dir(p.ConfigFile()), 0755))
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte(config), 0644))

	o, err := object.NewVol(p)
	require.NoError(t, err)
	var i any = o
	if lister, ok := i.(interface{ Resources() resource.Drivers }); ok {
		lister.Resources()
	}
	c, ok := i.(object.Configurer)
	require.True(t, ok)
	require.NoError(t, c.Config().Set(keyop.T{Key: key.New("DEFAULT", "size"), Op: keyop.Set, Value: "2mi"}))

	b, err := os.ReadFile(p.ConfigFile())
	require.NoError(t, err)
	assert.NotContains(t, string(b), "[node]")
	assert.Contains(t, string(b), "size = 2mi")
}
