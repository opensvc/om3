package network_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/testhelper"

	_ "github.com/opensvc/om3/v3/core/network"
	_ "github.com/opensvc/om3/v3/drivers/networklan"
)

func clusterAlerts(t *testing.T, conf string) []string {
	t.Helper()
	testhelper.Setup(t)
	c, err := object.NewCluster(object.WithConfigData([]byte(conf)))
	require.NoError(t, err)
	alerts, err := c.Config().Validate()
	require.NoError(t, err)
	l := make([]string, 0)
	for _, a := range alerts.Errors() {
		l = append(l, a.Key.String()+": "+a.Comment)
	}
	return l
}

const lanConf = `
[cluster]
nodes = n1 n2

[network#lan]
type = lan
network = 10.32.0.0/24
ranges = 10.32.0.200-10.32.0.239
`

// A cluster configuration whose networks hand out the same addresses, or a
// lan network whose ranges leave its segment, is invalid: the write making it
// is refused, the network setup refusing the network after it. The network
// declared first is not the one said invalid.
func TestClusterConfigRefusesNetworksOverlapping(t *testing.T) {
	assert.Empty(t, clusterAlerts(t, lanConf))

	l := clusterAlerts(t, lanConf+`
[network#bad]
type = lan
network = 10.32.0.0/24
ranges = 10.32.0.230-10.32.0.245
`)
	require.Len(t, l, 1)
	assert.True(t, strings.HasPrefix(l[0], "network#bad.ranges: "), l[0])
	assert.Contains(t, l[0], "overlaps 10.32.0.200-10.32.0.239 (lan)")

	l = clusterAlerts(t, lanConf+`
[network#out]
type = lan
network = 10.32.0.0/24
ranges = 10.33.0.200-10.33.0.210
`)
	require.Len(t, l, 1)
	assert.Contains(t, l[0], "is not in the segment")
}
