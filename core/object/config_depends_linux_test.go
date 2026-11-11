package object_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/capabilities"

	_ "github.com/opensvc/om3/v3/drivers/resipnetns"
)

// ignoredKeywords returns the ignored keyword alerts of the validation of a
// svc configuration, as "<key>: <comment>".
func ignoredKeywords(t *testing.T, conf string) []string {
	t.Helper()
	testhelper.Setup(t)
	caps := []string{driver.NewID(driver.GroupIP, "netns").Cap()}
	capabilities.Register(func(context.Context) ([]string, error) { return caps, nil })
	require.NoError(t, capabilities.Scan(t.Context()))
	p, err := naming.ParsePath("test/svc/web")
	require.NoError(t, err)
	o, err := object.NewSvc(p, object.WithVolatile(true), object.WithConfigData([]byte(conf)))
	require.NoError(t, err)
	alerts, err := o.Config().Validate()
	require.NoError(t, err)
	l := make([]string, 0)
	for _, a := range alerts {
		m := a.Unstructured()
		if m["kind"] == "ignored keyword" {
			l = append(l, m["key"].(string)+": "+m["comment"].(string))
		}
	}
	return l
}

// A keyword set where a condition it depends on does not hold is ignored by
// what reads it, which the validation warns of: the conditions are judged on
// the values the configuration reads, inherited and defaulted, and not judged
// on a key with no value, which only a start knows.
func TestValidateIgnoredKeywords(t *testing.T) {
	assert.Equal(t,
		[]string{"flex_min: flex_min is set, but it applies only where topology=flex, and topology is failover: it is ignored"},
		ignoredKeywords(t, "[DEFAULT]\nnodes = *\nflex_min = 2\n"), "a flex keyword of a failover service, by default")
	assert.Empty(t, ignoredKeywords(t, "[DEFAULT]\nnodes = *\ntopology = flex\nflex_min = 2\n"), "a flex keyword of a flex service")
	assert.Equal(t,
		[]string{"stonith: stonith is set, but it applies only where topology=failover, and topology is flex: it is ignored"},
		ignoredKeywords(t, "[DEFAULT]\nnodes = *\ntopology = flex\nstonith = true\n"), "a failover keyword of a flex service")

	assert.Empty(t, ignoredKeywords(t, "[DEFAULT]\nnodes = *\npg_cpus = 0-1\n"), "create_pg true by default")
	assert.Empty(t, ignoredKeywords(t, "[DEFAULT]\nnodes = *\ncreate_pg = yes\npg_cpus = 0-1\n"), "yes is true")
	assert.Equal(t,
		[]string{"pg_cpus: pg_cpus is set, but it applies only where create_pg=true, and create_pg is false: it is ignored"},
		ignoredKeywords(t, "[DEFAULT]\nnodes = *\ncreate_pg = false\npg_cpus = 0-1\n"), "a pg keyword with no pg")

	const ip = "[DEFAULT]\nnodes = *\n[container#1]\ntype = oci\nimage = ghcr.io/opensvc/pause\n[ip#1]\ntype = netns\nnetns = container#1\nname = 10.0.0.5\ndev = br0\nvlan_tag = 44\n"
	assert.Empty(t, ignoredKeywords(t, ip), "a mode a network chooses at start is not known")
	assert.Empty(t, ignoredKeywords(t, ip+"mode = ovs\n"), "an ovs vlan")
	assert.Equal(t,
		[]string{"ip#1.vlan_tag: vlan_tag is set, but it applies only where mode=ovs, and mode is bridge: it is ignored"},
		ignoredKeywords(t, ip+"mode = bridge\n"), "a vlan of a bridge")
}
