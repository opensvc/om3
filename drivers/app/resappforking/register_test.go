package resappforking

import (
	"github.com/opensvc/testhelper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/test_conf_helper"
	"os"
	"path/filepath"
	"testing"
	"time"
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
	td, cleanup := testhelper.Tempdir(t)
	defer cleanup()

	config.Load(map[string]string{"osvc_root_path": td})
	defer config.Load(map[string]string{})

	etc := filepath.Join(td, "etc")
	require.Nil(t, os.MkdirAll(etc, 0700))

	test_conf_helper.InstallSvcFile(t, "svc1.conf", filepath.Join(etc, "svc1.conf"))
	p, err := path.New("svc1", "", "")
	require.Nil(t, err)
	resources := object.NewSvc(p).Resources()

	t.Run("check default keywords value", func(t *testing.T) {
		app := getAppRid("app#1", resources)

		require.NotNil(t, app)
		require.Equal(t, "app#1", app.ResourceID.Name)
		assert.Nil(t, app.Timeout)
		assert.Nil(t, app.StartTimeout)
		assert.Nil(t, app.StopTimeout)
		assert.Equal(t, "/bin/true", app.StartCmd)
		assert.Equal(t, "", app.ScriptPath)
		assert.Equal(t, "", app.StopCmd, "")
		assert.Equal(t, "", app.CheckCmd, "")
		assert.Equal(t, "0:up 1:down", app.RetCodes)
		assert.Equal(t, []string{"[]"}, app.Env)
		assert.Equal(t, []string{"[]"}, app.SecretEnv)
		assert.Nil(t, app.Umask)
		assert.Equal(t, false, app.StatusLogKw)
		assert.Equal(t, "", app.Cwd)
		assert.Equal(t, "", app.User)
		assert.Equal(t, "", app.Group)
		assert.Nil(t, app.LimitCpu)
		assert.Nil(t, app.LimitAs)
		assert.Nil(t, app.LimitCore)
		assert.Nil(t, app.LimitData)
		assert.Nil(t, app.LimitFSize)
		assert.Nil(t, app.LimitMemLock)
		assert.Nil(t, app.LimitNoFile)
		assert.Nil(t, app.LimitNProc)
		assert.Nil(t, app.LimitRss)
		assert.Nil(t, app.LimitStack)
		assert.Nil(t, app.LimitVMem)

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
		assert.Equal(t, []string{"FOO_SEC=foo_sec", "BAR_SEC=bar_sec"}, app.SecretEnv)
		assert.Equal(t, "-----w--w-", app.Umask.String())
		assert.Equal(t, true, app.StatusLogKw)
		assert.Equal(t, "/tmp/foo", app.Cwd)
		assert.Equal(t, "foo", app.User)
		assert.Equal(t, "bar", app.Group)
		assert.Equal(t, 5*time.Minute+10*time.Second, *(app.LimitCpu))
		assert.Equal(t, int64(17*1000*1000), *(app.LimitAs))
		assert.Equal(t, int64(2*1000), *(app.LimitCore))
		assert.Equal(t, int64(2*1024*1024), *(app.LimitData))
		assert.Equal(t, int64(2.2*1000*1000*1000), *(app.LimitFSize))
		assert.Equal(t, int64(2.5*1024*1024*1024*1024), *(app.LimitMemLock))
		assert.Equal(t, int64(128), *(app.LimitNoFile))
		assert.Equal(t, int64(1500), *(app.LimitNProc))
		assert.Equal(t, int64(3*1024*1024*1024*1024*1024), *(app.LimitRss))
		assert.Equal(t, int64(9*1000*1000*1000*1000*1000*1000), *(app.LimitStack))
		assert.Equal(t, int64(7.5*1024*1024*1024*1024*1024*1024), *(app.LimitVMem))
	})
}
