package integrationtest

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/event"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/om"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/hostname"
)

func Test_Setup(t *testing.T) {
	if runtime.GOOS != "darwin" && os.Getuid() != 0 {
		t.Skip("skipped for non root user")
	}
	_, cancel := Setup(t)
	defer cancel()
}

func Test_GetClient(t *testing.T) {
	t.Logf("create client")
	_, err := GetClient(t)
	require.Nil(t, err)
}

func Test_daemon(t *testing.T) {
	if runtime.GOOS != "darwin" && os.Getuid() != 0 {
		t.Skip("skipped for non root user")
	}
	env, cancel := Setup(t)
	defer cancel()

	cData, err := GetDaemonStatus(t)
	require.Nil(t, err)

	paths := []naming.Path{naming.Cluster, naming.SecCa, naming.SecCert}
	for _, p := range paths {
		s := p.String()
		t.Run("check instance "+s, func(t *testing.T) {
			inst, ok := cData.Cluster.Node["node1"].Instance[s]
			assert.Truef(t, ok, "unable to find node1 instance %s", s)
			t.Logf("instance %s config: %+v", p, inst.Config)
			if s == "cluster" {
				require.NotNilf(t, inst.Config, "instance config should be defined for %s", s)
				if inst.Config != nil {
					require.Equal(t, []string{"node1", "node2", "node3"}, inst.Config.Scope)
				}
			}
		})
	}

	t.Run("check freeze when rejoin duration exceeded", func(t *testing.T) {
		t.Logf("ensure node frozen file absent before test")
		require.NoFileExists(t, filepath.Join(env.Root, "var", "node", "frozen"), "node frozen file should not exist")
		cli, err := GetClient(t)
		require.Nil(t, err)

		timeout := 2 * time.Second
		filters := []string{"NodeMonitorUpdated,node=node1"}
		readCloser, err := cli.NewGetEvents().SetFilters(filters).SetDuration(timeout).GetReader()
		require.NoError(t, err)
		defer func() {
			require.Nil(t, readCloser.Close())
		}()
		t.Logf("Install node.conf with reduced rejoin_grace_period")
		env.InstallFile("./testdata/node_with_reduced_rejoin_grace_period.conf", "etc/node.conf")
		waitNodeMonitorStates(t, readCloser, node.MonitorStateIdle)
		t.Logf("ensure node frozen file is now created")
		require.FileExistsf(t, filepath.Join(env.Root, "var", "node", "frozen"),
			"node frozen file should exist because of rejoin duration exceeded")
	})
	require.False(t, t.Failed(), "abort test")

	t.Run("drain orchestration when no svc objects", func(t *testing.T) {
		require.Nil(t, os.Setenv("OSVC_ROOT_PATH", env.Root))
		timeout := 2 * time.Second
		apiCall := func() (*http.Response, error) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			cli, err := GetClient(t)
			require.Nil(t, err)
			return cli.PostPeerActionDrain(ctx, hostname.Hostname())
		}
		checkRun(t, timeout, apiCall, http.StatusOK, node.MonitorStateDrainProgress, node.MonitorStateDrainSuccess, node.MonitorStateIdle)
	})
	require.False(t, t.Failed(), "abort test")

	t.Run("discover newly created object", func(t *testing.T) {
		env.InstallFile("./testdata/foo.conf", "etc/foo.conf")
		t.Logf("foo.conf created, wait 500ms and GetDaemonStatus to verify existence of instance foo")
		time.Sleep(500 * time.Millisecond)
		cData, err := GetDaemonStatus(t)
		require.Nil(t, err)
		p := naming.Path{Name: "foo", Kind: naming.KindSvc}
		_, ok := cData.Cluster.Node["node1"].Instance[p.String()]
		assert.Truef(t, ok, "unable to find node1 instance %s", p)
	})

	t.Run("drain orchestration when object svc exists", func(t *testing.T) {
		// It should be run with at least on svc object
		require.Nil(t, os.Setenv("OSVC_ROOT_PATH", env.Root))
		timeout := 2 * time.Second
		apiCall := func() (*http.Response, error) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			cli, err := GetClient(t)
			require.Nil(t, err)
			return cli.PostPeerActionDrain(ctx, hostname.Hostname())
		}
		checkRun(t, timeout, apiCall, http.StatusOK, node.MonitorStateDrainProgress, node.MonitorStateDrainSuccess, node.MonitorStateIdle)
	})
	require.False(t, t.Failed(), "abort test")
}

func checkRun(t *testing.T, timeout time.Duration, reqFunc func() (*http.Response, error), statusCode int, states ...node.MonitorState) {
	cli, err := GetClient(t)
	require.Nil(t, err)

	filters := []string{"NodeMonitorUpdated,node=node1"}
	readCloser, err := cli.NewGetEvents().SetFilters(filters).SetDuration(timeout).GetReader()
	require.NoError(t, err)
	defer func() { require.NoError(t, readCloser.Close()) }()
	resp, err := reqFunc()
	require.NoError(t, err)
	require.Equalf(t, statusCode, resp.StatusCode, "body: %s", resp.Body)
	t.Logf("wait for node status drained: %s", states)
	waitNodeMonitorStates(t, readCloser, states...)
	require.False(t, t.Failed(), "abort test")
}

func TestMain(m *testing.M) {
	testhelper.Main(m, om.ExecuteArgs)
}

func waitNodeMonitorStates(t *testing.T, evReader event.Reader, states ...node.MonitorState) {
	t.Helper()
	t.Logf("wait for node monitor states: %s", states)
	for _, state := range states {
		for {
			ev, err := evReader.Read()
			require.NoError(t, err, "unable to get node monitor update event")
			data := msgbus.NodeMonitorUpdated{}
			err = json.Unmarshal(ev.Data, &data)
			require.NoError(t, err, "unable to unmarshal node monitor update event")
			t.Logf("%s: got node monitor updated event: %s state: %s, local expect: %s",
				t.Name(), data.Node, data.Value.State, data.Value.LocalExpect)
			if data.Value.State == state {
				t.Logf("%s: node reach expected state %s", t.Name(), state)
				break
			}
		}
	}
	t.Logf("all node monitor states has been reached: %s", states)
}
