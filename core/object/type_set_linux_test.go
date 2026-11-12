package object_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/testhelper"

	_ "github.com/opensvc/om3/v3/core/driverdb"
)

// A write setting the type of a section to a driver om does not know is
// refused: the section was accepted with a warning, and ignored, as a pool
// missing from "om pool ls" with nothing saying why (card 266). A type the
// type keyword offers is accepted, and a section already holding an unknown
// type leaves the configuration writable.
func TestAWriteSettingAnUnknownTypeIsRefused(t *testing.T) {
	testhelper.Setup(t)
	cluster := func(conf string) *xconfig.T {
		c, err := object.NewCluster(object.WithConfigData([]byte(conf)), object.WithVolatile(true))
		require.NoError(t, err)
		return c.Config()
	}
	set := func(cfg *xconfig.T, ops ...string) error {
		return cfg.PrepareSet(keyop.ParseOps(ops)...)
	}

	err := set(cluster("[cluster]\nnodes = n1\n"), "pool#x.type=bogus")
	require.ErrorIs(t, err, xconfig.ErrUnknownType)
	assert.ErrorContains(t, err, "pool#x.type: no pool driver is named bogus, the known ones are ")
	assert.ErrorContains(t, err, "directory")

	require.NoError(t, set(cluster("[cluster]\nnodes = n1\n"), "pool#x.type=directory"))
	require.NoError(t, set(cluster("[cluster]\nnodes = n1\n"), "pool#x.type=share"), "a type the keyword offers")
	require.NoError(t, set(cluster("[cluster]\nnodes = n1\n[pool#old]\ntype = bogus\n"), "pool#x.type=directory", "cluster.name=c1"),
		"a section already holding an unknown type leaves the configuration writable")

	p, err := naming.ParsePath("test/svc/web")
	require.NoError(t, err)
	svc, err := object.NewSvc(p, object.WithConfigData([]byte("[DEFAULT]\nnodes = n1\n")), object.WithVolatile(true))
	require.NoError(t, err)
	cfg := svc.Config()
	require.ErrorIs(t, set(cfg, "fs#1.type=bogus"), xconfig.ErrUnknownType)
	require.ErrorIs(t, set(cfg, "volume#1.type=bogus"), xconfig.ErrUnknownType, "the type of a volume is the one of its pool")
	require.NoError(t, set(cfg, "fs#1.type=flag", "volume#1.type=directory"))
}
