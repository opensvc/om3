package resiphost_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/hostname"

	_ "github.com/opensvc/om3/v3/drivers/resiphost"
)

// strategyAlerts returns the conflicting strategies alerts of the validation
// of the configuration of a svc, as "<level> <key>: <comment>".
func strategyAlerts(t *testing.T, conf string) []string {
	t.Helper()
	p := naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "web"}
	o, err := object.NewSvc(p, object.WithConfigData([]byte("[DEFAULT]\nnodes = "+hostname.Hostname()+"\n"+conf)), object.WithVolatile(true))
	require.NoError(t, err)
	alerts, err := o.Config().Validate()
	require.NoError(t, err)
	l := make([]string, 0)
	for _, a := range alerts {
		m := a.Unstructured()
		if m["kind"] != "conflicting strategies" {
			continue
		}
		l = append(l, m["level"].(string)+" "+m["key"].(string)+": "+m["comment"].(string))
	}
	return l
}

// An ip addressed both statically and from a network is warned of, once per
// pair of keywords, whatever the scope or the alias they are written by. One
// way alone is not, nor the netmask of a static address overriding the one of
// a network, which the ipam strategy does not exclude.
func TestValidateConflictingStrategies(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)
	require.NoError(t, capabilities.Scan(t.Context()), "a section of a driver this node can not use is not validated further")

	assert.Equal(t,
		[]string{"warning ip#1.network: name and network are both set, but name is the static addressing strategy and network the ipam one"},
		strategyAlerts(t, "[ip#1]\ntype = host\nname = 1.2.3.4\nnetwork = default\n"))

	assert.Empty(t, strategyAlerts(t, "[ip#1]\ntype = host\nname = 1.2.3.4\n"), "name alone")
	assert.Empty(t, strategyAlerts(t, "[ip#1]\ntype = host\nnetwork = default\n"), "network alone")
	assert.Empty(t, strategyAlerts(t, "[ip#1]\ntype = host\nnetwork = default\nnetmask = 24\n"), "a netmask overriding the one of the network")
	assert.Empty(t, strategyAlerts(t, "[DEFAULT]\nname = 1.2.3.4\n[ip#1]\ntype = host\nnetwork = default\n"), "a value inherited from DEFAULT")

	l := strategyAlerts(t, "[ip#1]\nipname@"+hostname.Hostname()+" = 1.2.3.4\nnetwork = default\naddr = 1.2.3.5\n")
	assert.ElementsMatch(t, []string{
		"warning ip#1.network: ipname and network are both set, but ipname is the static addressing strategy and network the ipam one",
		"warning ip#1.addr: ipname and addr are both set, but ipname is the static addressing strategy and addr the ipam one",
	}, l, "the default driver of the section, a scoped alias")
}
