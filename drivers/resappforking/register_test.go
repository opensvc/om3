package resappforking

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/testhelper"

	// because used object has a fs flag resource
	_ "github.com/opensvc/om3/drivers/resfsflag"
)

func getAppRid(rid string, resources []resource.Driver) *T {
	for _, res := range resources {
		switch r := res.(type) {
		case *T:
			if r.ResourceID.Name == rid {
				return res.(*T)
			}
		}
	}
	return nil
}

func TestKeywords(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	env.InstallFile("test-fixtures/svc1.conf", "etc/svc1.conf")
	object.SetClusterConfig()
	defer rawconfig.ReloadForTest(env.Root)()
	p, err := naming.ParsePath("svc1")
	require.Nil(t, err)
	o, err := object.NewSvc(p)
	require.Nil(t, err)

	resources := o.Resources()
	require.Greater(t, len(resources), 0, "empty resource list for service")

	t.Run("check default keywords value", func(t *testing.T) {
		app := getAppRid("app#1", resources)

		require.NotNil(t, app)
		require.Equal(t, "app#1", app.ResourceID.Name)
		assert.Nil(t, app.Timeout)
		assert.Nil(t, app.StartTimeout)
		assert.Nil(t, app.StopTimeout)
		assert.Equal(t, "/bin/true", app.StartCmd)
		assert.Equal(t, "", app.ScriptPath)
		assert.Equal(t, "", app.StopCmd)
		assert.Equal(t, "", app.CheckCmd)
		assert.Equal(t, "0:up 1:down", app.RetCodes)
		assert.Equal(t, []string{}, app.Env)
		assert.Equal(t, []string{}, app.SecretsEnv)
		assert.Equal(t, []string{}, app.ConfigsEnv)
		assert.Nil(t, app.Umask)
		assert.Equal(t, false, app.StatusLogKw)
		assert.Equal(t, "", app.Cwd)
		assert.Equal(t, "", app.User)
		assert.Equal(t, "", app.Group)
		assert.Nil(t, app.Limit.CPU)
		assert.Nil(t, app.Limit.AS)
		assert.Nil(t, app.Limit.Core)
		assert.Nil(t, app.Limit.Data)
		assert.Nil(t, app.Limit.FSize)
		assert.Nil(t, app.Limit.MemLock)
		assert.Nil(t, app.Limit.NoFile)
		assert.Nil(t, app.Limit.NProc)
		assert.Nil(t, app.Limit.RSS)
		assert.Nil(t, app.Limit.Stack)
		assert.Nil(t, app.Limit.VMem)

	})

	t.Run("check custom keywords", func(t *testing.T) {
		app := getAppRid("app#2", resources)
		require.NotNil(t, app)
		require.Equal(t, "app#2", app.ResourceID.Name)
		assert.Equal(t, "scriptValue", app.ScriptPath)
		assert.Equal(t, "/path1/demo.sh start 106", app.StartCmd)
		assert.Equal(t, "/path1/demo.sh stop 106", app.StopCmd)
		assert.Equal(t, "/path2/demo.sh status", app.CheckCmd)
		assert.Equal(t, 3*time.Minute+10*time.Second, *(app.Timeout))
		assert.Equal(t, 1*time.Minute, *(app.StartTimeout))
		assert.Equal(t, 2*time.Minute, *(app.StopTimeout))
		assert.Equal(t, "1:up 0:down 3:n/a", app.RetCodes)
		assert.Equal(t, []string{"FOO=foo", "BAR=bar"}, app.Env)
		assert.Equal(t, []string{"FOO_SEC=foo_sec", "BAR_SEC=bar_sec"}, app.SecretsEnv)
		assert.Equal(t, "-----w--w-", app.Umask.String())
		assert.Equal(t, true, app.StatusLogKw)
		assert.Equal(t, "/tmp/foo", app.Cwd)
		assert.Equal(t, "foo", app.User)
		assert.Equal(t, "bar", app.Group)
		assert.Equal(t, 5*time.Minute+10*time.Second, *(app.Limit.CPU))
		assert.Equal(t, int64(17*1000*1000), *(app.Limit.AS))
		assert.Equal(t, int64(2*1000), *(app.Limit.Core))
		assert.Equal(t, int64(2*1024*1024), *(app.Limit.Data))
		assert.Equal(t, int64(2.2*1000*1000*1000), *(app.Limit.FSize))
		assert.Equal(t, int64(2.5*1024*1024*1024*1024), *(app.Limit.MemLock))
		assert.Equal(t, int64(128), *(app.Limit.NoFile))
		assert.Equal(t, int64(1500), *(app.Limit.NProc))
		assert.Equal(t, int64(3*1024*1024*1024*1024*1024), *(app.Limit.RSS))
		assert.Equal(t, int64(9*1000*1000*1000*1000*1000*1000), *(app.Limit.Stack))
		assert.Equal(t, int64(7.5*1024*1024*1024*1024*1024*1024), *(app.Limit.VMem))
		assert.Equal(t, "blocking post start", app.BlockingPostStart)
		assert.Equal(t, "blocking post stop", app.BlockingPostStop)
		assert.Equal(t, "post stop", app.PostStop)
		assert.Equal(t, "blocking post provision", app.BlockingPostProvision)
	})
}
