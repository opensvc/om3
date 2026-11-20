package icfg

import (
	"strings"
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

// A reference the node can't resolve leaves no datastore name in the line:
// the instance uses no datastore through it, and the install command,
// reading the same line, installs from none either.
func TestGetUsesOfAnUnresolvedReference(t *testing.T) {
	p, err := naming.ParsePath("repro/svc/d")
	require.NoError(t, err)
	for _, ref := range []string{"{env.missing}", "{missing.kw}"} {
		conf := "[DEFAULT]\nnodes = n1\n\n[volume#1]\nname = data\ninstall = /etc/x from ./cfg/" + ref + " key k\n"
		oc, err := object.NewConfigurer(p, object.WithConfigData([]byte(conf)), object.WithVolatile(true))
		require.NoError(t, err)
		m := &Manager{path: p}
		assert.Equal(t, []string{"repro/vol/data"}, m.getUses(oc.Config()), ref)
	}
}

// Each resource publishes what it uses, as the object does for all of them:
// a resource enabled again says the datastores it starts using even when
// another resource uses them already. A disabled resource uses nothing.
func TestGetResourcesSayWhatEachUses(t *testing.T) {
	p, err := naming.ParsePath("repro/svc/d")
	require.NoError(t, err)
	conf := `
[DEFAULT]
nodes = n1

[volume#1]
name = data
install = /k1.conf from ./cfg/demo key k1.conf

[volume#2]
name = data2
install = /k1.conf from ./cfg/demo key k1.conf
disable = true

[app#1]
environment = DB_PASSWORD from ./sec/db key password
`
	oc, err := object.NewConfigurer(p, object.WithConfigData([]byte(conf)), object.WithVolatile(true))
	require.NoError(t, err)
	m := &Manager{path: p}
	defer hostname.Impersonate("n1")()

	resources := m.getResources(oc.Config())
	assert.Equal(t, []string{"repro/cfg/demo", "repro/vol/data"}, resources["volume#1"].Uses)
	assert.Empty(t, resources["volume#2"].Uses, "a disabled resource uses nothing")
	assert.Equal(t, []string{"repro/sec/db"}, resources["app#1"].Uses)
	assert.Equal(t, []string{"repro/cfg/demo", "repro/sec/db", "repro/vol/data"}, m.getUses(oc.Config()),
		"the object uses what its resources do")

	enabled, err := object.NewConfigurer(p, object.WithConfigData([]byte(strings.Replace(conf, "disable = true", "disable = false", 1))), object.WithVolatile(true))
	require.NoError(t, err)
	assert.Equal(t, []string{"repro/cfg/demo", "repro/vol/data2"}, m.getResources(enabled.Config())["volume#2"].Uses,
		"enabled again, it uses the datastore volume#1 uses already")
	assert.Equal(t, []string{"repro/cfg/demo", "repro/sec/db", "repro/vol/data", "repro/vol/data2"}, m.getUses(enabled.Config()),
		"while the object only starts using the vol, not the datastore")
}
