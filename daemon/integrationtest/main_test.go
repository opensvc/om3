package integrationtest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/om"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/testhelper"
	"github.com/opensvc/om3/util/hostname"
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

	paths := []string{"cluster", "system/sec/ca", "system/sec/cert"}
	for _, p := range paths {
		t.Run("check instance "+p, func(t *testing.T) {
			inst, ok := cData.Cluster.Node["node1"].Instance[p]
			assert.Truef(t, ok, "unable to find node1 instance %s", p)
			t.Logf("instance %s config: %+v", p, inst.Config)
			if p == "cluster" {
				require.NotNilf(t, inst.Config, "instance config should be defined for %s", p)
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
		checkRun(t, timeout, apiCall, http.StatusOK, node.MonitorStateDraining, node.MonitorStateDrained, node.MonitorStateIdle)
	})
	require.False(t, t.Failed(), "abort test")

	t.Run("discover newly created object", func(t *testing.T) {
		env.InstallFile("./testdata/foo.conf", "etc/foo.conf")
		time.Sleep(250 * time.Millisecond)
		cData, err := GetDaemonStatus(t)
		p := naming.Path{Name: "foo", Kind: naming.KindSvc}
		require.Nil(t, err)
		_, ok := cData.Cluster.Node["node1"].Instance[p.String()]
		assert.Truef(t, ok, "unable to find node1 instance %s", p)
	})

	// node should be frozen for this test, we don't want orchestration on created vip object
	t.Run("cluster.vip with a frozen node", func(t *testing.T) {
		var (
			cfgUpdateReader, setInstanceReader, imonUpdateReader event.ReadCloser
		)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// need enough time for test with race
		maxTestDuration := 3 * time.Second
		cli, err := GetClient(t)
		require.Nil(t, err)
		require.FileExistsf(t, filepath.Join(env.Root, "var", "node", "frozen"),
			"test should start with a frozen node")
		cfgUpdateReader, err = cli.NewGetEvents().
			SetFilters([]string{"InstanceConfigUpdated,path=system/svc/vip"}).
			SetDuration(maxTestDuration).
			GetReader()
		require.NoError(t, err)
		setInstanceReader, err = cli.NewGetEvents().
			SetFilters([]string{"SetInstanceMonitor,path=system/svc/vip"}).
			SetDuration(maxTestDuration).
			GetReader()
		require.NoError(t, err)
		imonUpdateReader, err = cli.NewGetEvents().
			SetFilters([]string{"InstanceMonitorUpdated,path=system/svc/vip"}).
			SetDuration(maxTestDuration).
			GetReader()
		require.NoError(t, err)
		defer func() {
			require.NoError(t, cfgUpdateReader.Close())
			require.NoError(t, setInstanceReader.Close())
			require.NoError(t, imonUpdateReader.Close())
		}()
		kwName := "cluster.vip"
		t.Run("post object config update on cluster", func(t *testing.T) {
			params := api.PostObjectConfigUpdateParams{
				Set: &api.InQuerySets{
					fmt.Sprintf("%s=%s", kwName, "foo/24@default"),
					fmt.Sprintf("%s@node1=%s", kwName, "foo1/32@custom1"),
					fmt.Sprintf("%s@node3=%s", kwName, "foo3/32@custom3"),
				},
			}
			resp, err := cli.PostObjectConfigUpdateWithResponse(ctx,
				"root", "ccfg", "cluster", &params)
			require.NoError(t, err)
			require.Equalf(t, http.StatusNoContent, resp.StatusCode(), "body: %s", resp.Body)
		})
		require.False(t, t.Failed(), "abort test")

		t.Run("wait for system/svc/vip object automatically created", func(t *testing.T) {
			t.Log("wait for instance config event for system/svc/vip")
			cfgEv, err := cfgUpdateReader.Read()
			require.NoError(t, err, "unable to get system/svc/vip config update event")
			t.Logf("got instance config event: %s", cfgEv.Render())
		})
		require.False(t, t.Failed(), "abort test")

		t.Run("wait for system/svc/vip thawed global expect", func(t *testing.T) {
			t.Log("wait for instance monitor event thawed for system/svc/vip")
			for {
				ev, err := imonUpdateReader.Read()
				require.NoError(t, err, "unable to get system/svc/vip imon update event")
				data := msgbus.InstanceMonitorUpdated{}
				err = json.Unmarshal(ev.Data, &data)
				require.NoError(t, err, "unable to unmarshal imon update event")
				t.Logf("got instance imon event: %s@%s state: %s global expect:%s",
					data.Path, data.Node, data.Value.State, data.Value.GlobalExpect)
				if data.Value.GlobalExpect == instance.MonitorGlobalExpectThawed {
					t.Logf("done wait: got expected global expect thawed")
					break
				}
			}
		})
		require.False(t, t.Failed(), "abort test")

		t.Run("GetObjectConfigGet on vip object", func(t *testing.T) {
			ptBool := func(b bool) *api.InQueryEvaluate { return &b }
			ptString := func(s string) *api.InQueryImpersonate { return &s }

			for name, tc := range map[string]struct {
				param    api.GetObjectConfigGetParams
				expected string
			}{
				"ip#0.ipname is created from ipname of default cluster.vip": {
					param: api.GetObjectConfigGetParams{
						Kw:       &api.InQueryKeywords{"ip#0.ipname"},
						Evaluate: ptBool(false),
					},
					expected: "foo",
				},

				"ip#0.netmask is created from netmask of default cluster.vip": {
					param: api.GetObjectConfigGetParams{
						Kw:       &api.InQueryKeywords{"ip#0.netmask"},
						Evaluate: ptBool(false),
					},
					expected: "24",
				},

				"ip#0.ipdev is created from dev of default cluster.vip": {
					param: api.GetObjectConfigGetParams{
						Kw:       &api.InQueryKeywords{"ip#0.ipdev"},
						Evaluate: ptBool(false),
					},
					expected: "default",
				},

				"cluster.vip@... is not used for ip#0.ipname (evaluate & impersonate)": {
					param: api.GetObjectConfigGetParams{
						Kw:          &api.InQueryKeywords{"ip#0.ipname"},
						Impersonate: ptString("node1"),
						Evaluate:    ptBool(true),
					},
					expected: "foo",
				},

				"cluster.vip@... is not used for ip#0.ipname (evaluate)": {
					param: api.GetObjectConfigGetParams{
						Kw:       &api.InQueryKeywords{"ip#0.ipname"},
						Evaluate: ptBool(true),
					},
					expected: "foo",
				},

				"cluster.vip@... is not used for ip#0.netmask (evaluate & impersonate)": {
					param: api.GetObjectConfigGetParams{
						Kw:          &api.InQueryKeywords{"ip#0.netmask"},
						Impersonate: ptString("node1"),
						Evaluate:    ptBool(true),
					},
					expected: "24"},

				"cluster.vip@... is not used for ip#0.netmask (evaluate)": {
					param: api.GetObjectConfigGetParams{
						Kw:       &api.InQueryKeywords{"ip#0.netmask"},
						Evaluate: ptBool(true),
					},
					expected: "24",
				},

				"cluster.vip@... is used for ip#0.ipdev (evaluate & impersonate node1)": {
					param: api.GetObjectConfigGetParams{
						Kw:          &api.InQueryKeywords{"ip#0.ipdev"},
						Evaluate:    ptBool(true),
						Impersonate: ptString("node1"),
					},
					expected: "custom1",
				},

				"cluster.vip@... is used for ip#0.ipdev (evaluate & impersonate node3)": {
					param: api.GetObjectConfigGetParams{
						Kw:          &api.InQueryKeywords{"ip#0.ipdev"},
						Evaluate:    ptBool(true),
						Impersonate: ptString("node3"),
					},
					expected: "custom3",
				},

				"cluster.vip@... is used for ip#0.ipdev (evaluate)": {
					param: api.GetObjectConfigGetParams{
						Kw:       &api.InQueryKeywords{"ip#0.ipdev"},
						Evaluate: ptBool(true),
					},
					expected: "custom1",
				},
			} {
				param := tc.param
				expected := tc.expected
				t.Run(name, func(t *testing.T) {
					resp, err := cli.GetObjectConfigGetWithResponse(ctx, "system", "svc", "vip", &param)
					require.NoError(t, err)
					require.Equalf(t, http.StatusOK, resp.StatusCode(), "body: %s", resp.Body)
					require.Equalf(t, tc.expected, resp.JSON200.Items[0].Data.Value,
						"%s value not found in items: %#v", expected, resp.JSON200.Items)
				})

			}
		})
		require.False(t, t.Failed(), "abort test")
		t.Log("Test")
		t.Run("get /node/name/{nodename}/system/... must return 404 if package cache not yet present", func(t *testing.T) {
			testCases := map[string]func(context.Context, string, ...api.RequestEditorFn) (*http.Response, error){
				"package":       cli.GetNodeSystemPackage,
				"patch":         cli.GetNodeSystemPatch,
				"disk":          cli.GetNodeSystemDisk,
				"group":         cli.GetNodeSystemGroup,
				"hardware":      cli.GetNodeSystemHardware,
				"ipaddress":     cli.GetNodeSystemIPAddress,
				"property":      cli.GetNodeSystemProperty,
				"san/initiator": cli.GetNodeSystemSANInitiator,
				"san/path":      cli.GetNodeSystemSANPath,
				"user":          cli.GetNodeSystemUser,
			}
			for s, f := range testCases {
				t.Run("GET /node/name/{nodename}/system/"+s, func(t *testing.T) {

					resp, err := f(ctx, hostname.Hostname())
					require.NoError(t, err, "unexpected error during cli.GetNodeSystemPackageWithResponse")
					require.Equalf(t, http.StatusNotFound, resp.StatusCode, "body: %s", resp.Body)
				})
			}
		})
		require.False(t, t.Failed(), "abort test")

		t.Run("get /node/name/{nodename}/system/package must return package cache if present", func(t *testing.T) {
			env.InstallFile("./testdata/package.json", "var/node/package.json")
			resp, err := cli.GetNodeSystemPackageWithResponse(ctx, hostname.Hostname())
			require.NoError(t, err, "unexpected error during cli.GetNodeSystemPackageWithResponse")
			require.Equalf(t, http.StatusOK, resp.StatusCode(), "body: %s", resp.Body)
			require.Len(t, resp.JSON200.Items, 2)
			require.Equalf(t, "foo", resp.JSON200.Items[0].Data.Name, "can't find foo package")
		})
		require.False(t, t.Failed(), "abort test")

		t.Run("post object config update on cluster to delete cluster.vip", func(t *testing.T) {
			params := api.PostObjectConfigUpdateParams{
				Unset: &api.InQueryUnsets{kwName, kwName + "@node1", kwName + "@node2"},
			}
			resp, err := cli.PostObjectConfigUpdateWithResponse(ctx,
				"root", "ccfg", "cluster", &params)
			require.NoError(t, err, "can't post config update")
			require.Equalf(t, http.StatusNoContent, resp.StatusCode(), "body: %s", resp.Body)
		})
		require.False(t, t.Failed(), "abort test")

		t.Run("wait for system/svc/vip purge order", func(t *testing.T) {
			data := msgbus.SetInstanceMonitor{}
			for {
				ev, err := setInstanceReader.Read()
				require.NoError(t, err, "unable to get system/svc/vip purge order")
				require.NoError(t, json.Unmarshal(ev.Data, &data))
				if *data.Value.GlobalExpect == instance.MonitorGlobalExpectPurged {
					t.Logf("got %+v", data.Value)
					break
				}
			}
		})
		require.False(t, t.Failed(), "abort test")
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
		checkRun(t, timeout, apiCall, http.StatusOK, node.MonitorStateDraining, node.MonitorStateDrained, node.MonitorStateIdle)
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
