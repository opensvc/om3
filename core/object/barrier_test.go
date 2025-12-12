package object

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/testhelper"
)

func TestBarrierToFlag(t *testing.T) {
	testhelper.Setup(t)

	t.Run("to fs#2", func(t *testing.T) {

		env := testhelper.Setup(t)
		_ = env
		_, err := SetClusterConfig()
		require.NoError(t, err)

		cf := []byte(`
[fs#1]
type = flag

[fs#2]
type = flag

[fs#3]
type = flag
`)
		clusterConfig := cluster.Config{
			Name: "cluster1",
		}
		clusterConfig.SetSecret("9ceab2da-a126-4187-83f2-4900da8a6825")
		cluster.ConfigData.Set(&clusterConfig)

		p, _ := naming.ParsePath("test/svc/svc1")
		o, err := NewSvc(p, WithConfigData(cf))
		require.NoError(t, err)
		ctx := context.Background()
		ctx = actioncontext.WithTo(ctx, "fs#2")
		err = o.Start(ctx)
		require.NoError(t, err)
		svcStatus, err := o.Status(context.Background())
		require.NoError(t, err)
		resources := svcStatus.Resources
		for resource := range resources {
			if resource == "fs#3" {
				require.True(t, resources[resource].Status.Is(status.Down))
			} else {
				require.True(t, resources[resource].Status.Is(status.Up))
			}
		}
	})
}
