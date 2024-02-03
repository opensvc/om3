package integrationtest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/om"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/testhelper"
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

	t.Run("discover newly created object", func(t *testing.T) {
		env.InstallFile("./testdata/foo.conf", "etc/foo.conf")
		time.Sleep(250 * time.Millisecond)
		cData, err := GetDaemonStatus(t)
		p := naming.Path{Name: "foo", Kind: naming.KindSvc}
		require.Nil(t, err)
		_, ok := cData.Cluster.Node["node1"].Instance[p.String()]
		assert.Truef(t, ok, "unable to find node1 instance %s", p)
	})

	// because of multiple nodes, daemon should stay in rejoin state
	// => no orchestration on created vip object
	t.Run("cluster.vip when daemon is in rejoin state", func(t *testing.T) {
		var (
			cfgUpdateReader, setInstanceReader, imonUpdateReader event.ReadCloser
		)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		cli, err := GetClient(t)
		require.Nil(t, err)

		cfgUpdateReader, err = cli.NewGetEvents().
			SetFilters([]string{"InstanceConfigUpdated,path=system/svc/vip"}).
			SetDuration(time.Second).
			GetReader()
		require.NoError(t, err)
		setInstanceReader, err = cli.NewGetEvents().
			SetFilters([]string{"SetInstanceMonitor,path=system/svc/vip"}).
			SetDuration(time.Second).
			GetReader()
		require.NoError(t, err)
		imonUpdateReader, err = cli.NewGetEvents().
			SetFilters([]string{"InstanceMonitorUpdated,path=system/svc/vip"}).
			SetDuration(time.Second).
			GetReader()
		require.NoError(t, err)
		defer func() {
			require.NoError(t, cfgUpdateReader.Close())
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
}

func TestMain(m *testing.M) {
	testhelper.Main(m, om.ExecuteArgs)
}
