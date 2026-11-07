package rescontainerpodman_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/drivers/rescontainerocibase"
	"github.com/opensvc/om3/v3/drivers/rescontainerpodman"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/hostname"
)

// strategyAlerts returns the conflicting strategies alerts of the validation
// of the configuration of a svc, as "<level> <key>: <comment>".
func strategyAlerts(t *testing.T, conf string) []string {
	t.Helper()
	p := naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "db"}
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

// The quiesce of an oci container is chosen by the value of the quiesce
// keyword, pause or a command, which the strategies say by their When. Both
// use the quiesce keyword, so the keywords feeding a command are never warned
// of: neither by a command, which reads them, nor by pause, whose value the
// validation of the strategies does not read.
func TestQuiesceStrategies(t *testing.T) {
	g := rescontainerocibase.GroupQuiesce
	require.Len(t, g.Strategies, 2)
	pause, command := g.Strategies[0], g.Strategies[1]
	assert.Equal(t, "pause", pause.ID)
	require.Len(t, pause.When, 1)
	assert.Equal(t, "quiesce=pause", pause.When[0].String())
	assert.Equal(t, []string{"quiesce_input", "quiesce_ready", "resume_input", "quiesce_timeout"}, pause.Excludes)
	assert.Equal(t, "command", command.ID)
	require.Len(t, command.When, 1)
	assert.Equal(t, "quiesce!=pause", command.When[0].String())
	assert.Empty(t, command.Excludes)

	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)

	// The validation skips the sections of a driver the node can not use:
	// the node running the test may have no podman.
	drvCap := rescontainerpodman.DrvID.Cap()
	capabilities.Register(func(context.Context) ([]string, error) { return []string{drvCap}, nil })
	require.NoError(t, capabilities.Scan(t.Context()))

	assert.Empty(t, strategyAlerts(t, "[container#1]\ntype = podman\nquiesce = pause\n"), "pause")
	l := strategyAlerts(t, "[container#1]\ntype = podman\nquiesce = pause\nquiesce_input = FLUSH TABLES;\n")
	if assert.Len(t, l, 1, "pause reads no input") {
		assert.Contains(t, l[0], "quiesce_input")
	}
	assert.Empty(t, strategyAlerts(t, "[container#1]\ntype = podman\nquiesce = mariadb\nquiesce_input = FLUSH TABLES WITH READ LOCK; SELECT 'quiesced';\nquiesce_ready = quiesced\nresume_input = UNLOCK TABLES;\nquiesce_timeout = 1m\n"), "a command")
}
