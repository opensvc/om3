package object

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/testhelper"

	_ "github.com/opensvc/om3/v3/drivers/resfsflag"
)

// A section named with a path out of the var directory of the object is no
// resource: its driver would make its files there. The validation says so,
// as an error.
func TestASectionEscapingTheVarDirIsNoResource(t *testing.T) {
	testhelper.Setup(t)
	p, err := naming.ParsePath("test/svc/escape")
	require.NoError(t, err)
	o, err := New(p, WithConfigData([]byte(`[DEFAULT]
nodes = *

[fs#1]
type = flag

[fs#x/../../../../../../../../etc/cron.d]
type = flag
`)), WithVolatile(true))
	require.NoError(t, err)
	a := o.(*svc)
	a.ConfigureResources()
	require.Len(t, a.Resources(), 1)
	assert.Equal(t, "fs#1", a.Resources()[0].RID())

	alerts, err := a.ValidateConfig(context.Background())
	require.NoError(t, err)
	assert.True(t, alerts.HasError())
	assert.Contains(t, alerts.Errors().StringWithoutMeta(), "fs#x/../../../../../../../../etc/cron.d")
}
