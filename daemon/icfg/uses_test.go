package icfg

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/hostname"

	_ "github.com/opensvc/om3/v3/drivers/resappforking"
	_ "github.com/opensvc/om3/v3/drivers/resvol"
)

// Each node publishes the objects the instance uses as it evaluates the
// configuration: a keyword scoped to a node names what is used there, and a
// disabled resource uses nothing.
func TestGetUsesIsEvaluatedPerNode(t *testing.T) {
	p, err := naming.ParsePath("repro/svc/d")
	require.NoError(t, err)
	conf := `
[DEFAULT]
nodes = n1 n2
unprovision = false

[volume#1]
name = {name}-data
configs = d/*:/cfg
secrets = d/*:/sec
install@n2 = /etc/x from ./cfg/n2only key k

[volume#2]
disable = true
name = off

[app#1]
environment = DB_PASSWORD from ./sec/db key password
`
	oc, err := object.NewConfigurer(p, object.WithConfigData([]byte(conf)), object.WithVolatile(true))
	require.NoError(t, err)
	m := &Manager{path: p}

	usesOn := func(node string) []string {
		defer hostname.Impersonate(node)()
		return m.getUses(oc.Config())
	}
	assert.Equal(t, []string{"repro/cfg/d", "repro/sec/d", "repro/sec/db", "repro/vol/d-data"}, usesOn("n1"))
	assert.Equal(t, []string{"repro/cfg/d", "repro/cfg/n2only", "repro/sec/d", "repro/sec/db", "repro/vol/d-data"}, usesOn("n2"))
}
