package integrationtest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"opensvc.com/opensvc/cmd"
	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/testhelper"
)

func Test_Setup(t *testing.T) {
	t.Logf("Setup...")
	env, cancel := Setup(t)
	defer cancel()
	t.Logf("setup is done on env.Root: %s", env.Root)
}

func Test_GetClient(t *testing.T) {
	t.Logf("create client")
	_, err := GetClient(t)
	require.Nil(t, err)
}

func Test_GetDaemonStatus(t *testing.T) {
	env, cancel := Setup(t)
	defer cancel()

	cData, err := GetDaemonStatus(t)
	require.Nil(t, err)

	paths := []string{"cluster", "system/sec/ca-cluster1", "system/sec/cert-cluster1"}
	for _, p := range paths {
		t.Run("check instance "+p, func(t *testing.T) {
			inst, ok := cData.Cluster.Node["node1"].Instance[p]
			assert.Truef(t, ok, "unable to find node1 instance %s", p)
			t.Logf("instance %s config: %+v", p, inst.Config)
			if p == "cluster" {
				require.Equal(t, []string{"node1"}, inst.Config.Scope)
			}
		})
	}

	t.Run("discover newly created object", func(t *testing.T) {
		env.InstallFile("./testdata/foo.conf", "etc/foo.conf")
		time.Sleep(150 * time.Millisecond)
		cData, err := GetDaemonStatus(t)
		p := path.T{Name: "foo", Kind: kind.Svc}
		require.Nil(t, err)
		_, ok := cData.Cluster.Node["node1"].Instance[p.String()]
		assert.Truef(t, ok, "unable to find node1 instance %s", p)
	})
}

func TestMain(m *testing.M) {
	testhelper.Main(m, cmd.ExecuteArgs)
}
