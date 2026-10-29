package object_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/testhelper"

	_ "github.com/opensvc/om3/v3/drivers/resipcni"
)

// A resource refusing its configuration when the object loads it, by a check
// its driver makes beyond the keywords, is an error of the validation, a
// warning when it is disabled, and never started.
func TestValidateResources(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)
	p, err := naming.ParsePath("c1/svc/v")
	require.NoError(t, err)
	validate := func(cfg string) []map[string]any {
		t.Helper()
		o, err := object.NewSvc(p, object.WithConfigData([]byte(cfg)), object.WithVolatile(true))
		require.NoError(t, err)
		l := make([]map[string]any, 0)
		v, ok := any(o).(interface{ ValidateResources() xconfig.Alerts })
		require.True(t, ok)
		for _, a := range v.ValidateResources() {
			l = append(l, a.Unstructured())
		}
		return l
	}

	l := validate("[ip#0]\ntype = cni\nnetwork = nosuchnet\n")
	require.Len(t, l, 1)
	assert.Equal(t, "error", l[0]["level"])
	assert.Equal(t, "ip#0.", l[0]["key"], "the section, as the alerts of a section say it")
	assert.Equal(t, "resource configuration error", l[0]["kind"])
	assert.Contains(t, l[0]["comment"], "unknown network nosuchnet")

	l = validate("[ip#0]\ntype = cni\nnetwork = nosuchnet\ndisable = true\n")
	require.Len(t, l, 1)
	assert.Equal(t, "warning", l[0]["level"], "a disabled resource is never started")

	l = validate("[ip#0]\ntype = cni\nnetwork = ../../../tmp/x\n")
	require.Len(t, l, 1)
	assert.Contains(t, l[0]["comment"], "a network is named, not a path")

	assert.Empty(t, validate("[ip#0]\ntype = cni\nnetwork = default\n"), "the implicit default network")
}
