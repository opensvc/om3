package object_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"

	_ "github.com/opensvc/om3/v3/core/driverdb"
)

// A filesystem sized by its own driver, as a tmpfs or a quota-capped
// directory, names no volume group, so its size is not the om2 one the
// filesystem migration moves to a logical volume resource.
func TestAFilesystemSizedByItsOwnDriverMakesNoVolume(t *testing.T) {
	for _, config := range []string{
		"[fs#0]\ntype = tmpfs\ndev = none\nmnt = /srv/a\nsize = 1g\n",
		"[fs#0]\ntype = directory\npath = /srv/a\nsize = 1g\n",
	} {
		p, err := naming.ParsePath("test/svc/foo")
		require.NoError(t, err)
		o, err := object.NewSvc(p, object.WithConfigData([]byte(config)), object.WithVolatile(true))
		require.NoError(t, err)
		m := object.MigrateConfig(o.Config())
		for _, op := range m.Sets {
			assert.NotEqualf(t, "disk#0.type", op.Key.String(), "%s: no volume is made", config)
		}
		assert.Emptyf(t, m.Refusals, "%s", config)
	}
}
