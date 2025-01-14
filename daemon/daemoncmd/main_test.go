package daemoncmd_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/clusterdump"
	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/om"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/daemoncmd"
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/testhelper"
)

func newClient(serverUrl string) (*client.T, error) {
	return client.New(client.WithURL(serverUrl), client.WithPassword(cluster.ConfigData.Get().Secret()))
	//return client.New(client.WithURL(serverUrl), client.WithInsecureSkipVerify(true))
}

func setup(t *testing.T, withConfig bool) testhelper.Env {
	env := testhelper.Setup(t)
	if withConfig {
		env.InstallFile("./testdata/nodes_info.json", "var/nodes_info.json")
		env.InstallFile("./testdata/cluster.conf", "etc/cluster.conf")
		b, err := os.ReadFile("./testdata/cluster.conf")
		require.NoError(t, err)
		t.Logf("cluster.conf:\n%s\n", b)
		env.InstallFile("./testdata/ca-cluster1.conf", "etc/namespaces/system/sec/ca.conf")
		env.InstallFile("./testdata/cert-cluster1.conf", "etc/namespaces/system/sec/cert.conf")
	}
	return env
}

func getClientUrl(withConfig bool) (urlM map[string]string) {
	switch withConfig {
	case true:
		urlM = map[string]string{
			"UrlUxHttp":   daemonenv.HTTPUnixURL(),
			"UrlInetHttp": "https://localhost:1315",
		}
	case false:
		urlM = map[string]string{
			"UrlUxHttp":   daemonenv.HTTPUnixURL(),
			"UrlInetHttp": daemonenv.HTTPLocalURL(),
		}
	}
	return
}

func TestMain(m *testing.M) {
	testhelper.Main(m, om.ExecuteArgs)
}

func TestDaemonBootstrap(t *testing.T) {
	if runtime.GOOS != "darwin" && os.Getuid() != 0 {
		t.Skip("skipped for non root user")
	}
	for name, hasConfig := range map[string]bool{
		"from existing cluster node":  true,
		"from freshly installed node": false,
	} {
		t.Run(name, func(t *testing.T) {
			hasConfig := hasConfig
			env := setup(t, hasConfig)
			fixLog := testhelper.FixLogger()
			defer fixLog()
			t.Logf("using env root: %s", env.Root)
			cli, err := client.New(client.WithURL(daemonenv.HTTPUnixURL()))
			daemonCli := daemoncmd.New(cli)
			t.Logf("daemonCli.Running")
			isRunning, err := daemonCli.IsRunning()
			require.NoError(t, err)
			require.False(t, isRunning)
			startError := make(chan error)

			t.Logf("check daemonCli.Start")
			t.Run("check daemonCli.Start", func(t *testing.T) {
				t.Logf("daemonCli.Start...")
				go func() {
					startError <- daemonCli.Start()
				}()
				select {
				case err := <-startError:
					require.NoError(t, err, "daemonCli.Start() returns error !")
				case <-time.After(300 * time.Millisecond):
					t.Logf("daemonCli.Start() doesn't return error yet, it must be running...")
				}
				require.FileExists(t, filepath.Join(rawconfig.Paths.Var, "osvcd.pid"))
			})
			require.False(t, t.Failed(), "can't continue test: initial daemonCli.Start() has errors")

			t.Logf("daemonCli.WaitRunning")
			t.Run("daemonCli.WaitRunning", func(t *testing.T) {
				t.Logf("daemonCli.WaitRunning")
				require.NoError(t, daemonCli.WaitRunning())
			})

			t.Logf("check running")
			t.Run("check running", func(t *testing.T) {
				t.Logf("daemonCli.Running")
				isRunning, err = daemonCli.IsRunning()
				require.NoError(t, err)
				require.True(t, isRunning)
			})

			t.Logf("check events")
			t.Run("check events", func(t *testing.T) {
				// TODO move test get node events to other location asap
				//time.Sleep(150 * time.Millisecond)
				t.Logf("get node events")
				readEv, err := cli.NewGetEvents().
					SetLimit(1).
					SetDuration(1 * time.Second).
					GetReader()
				require.NoError(t, err)
				_, _ = cli.NewGetDaemonStatus().Get()
				events := make([]event.Event, 0)
				for {
					if ev, err := readEv.Read(); err != nil {
						t.Logf("readEv.Read error %s", err)
						break
					} else {
						t.Logf("read event %#v", *ev)
						events = append(events, *ev)
					}
				}
				if err := readEv.Close(); err != nil {
					t.Logf("readEv.Close err:%s", err)
				}
				require.Greaterf(t, len(events), 0, "no events returned !")
			})

			t.Logf("check daemon status with url %s", cli.URL())
			t.Run("check daemon status", func(t *testing.T) {
				t.Logf("get daemon status")
				t.Logf("give extra time for objects get pushed to daemon")
				time.Sleep(150 * time.Millisecond)

				var b []byte
				b, err = cli.NewGetDaemonStatus().Get()
				require.NoError(t, err)
				t.Logf("get daemon status response: %s", b)
				cData := clusterdump.Data{}
				err = json.Unmarshal(b, &cData)
				require.NoErrorf(t, err, "unmarshall daemon status response: %s", b)
				t.Logf("get daemon status response: %+v", cData)
				if hasConfig {
					require.Equal(t, "cluster1", cData.Cluster.Config.Name)
				} else {
					require.Greaterf(t, len(cData.Cluster.Config.Name), 2,
						"automatically defined cluster name is too short: %s", cData.Cluster.Config.Name)
					require.Contains(t, cData.Cluster.Config.Name, "-",
						"automatically random cluster name should contain '-', found: %s",
						cData.Cluster.Config.Name)
					// TODO: check for automatically defined heartbeat hb#1.type=unicast
				}
				for _, objectName := range []string{
					"system/sec/cert",
					"system/sec/ca",
					"cluster",
				} {
					t.Logf("search object %s", objectName)
					_, ok := cData.Cluster.Object[objectName]
					require.Truef(t, ok, "unable to detect object %s", objectName)
				}
			})

			for name, url := range getClientUrl(hasConfig) {
				t.Logf("check daemon status %s with url %s", name, url)
				t.Run("check running with client "+name, func(t *testing.T) {
					cli, err := newClient(url)
					require.NoError(t, err)
					isRunning, err = daemoncmd.New(cli).IsRunning()
					require.NoError(t, err)
					require.Truef(t, isRunning, "can't detect running from client with url %s", url)
				})
			}

			t.Logf("stopping")
			t.Run("stopping", func(t *testing.T) {
				t.Run("daemoncli stop", func(t *testing.T) {
					t.Logf("daemonCli.Stop...")
					// Use UrlInetHttp to avoid failed stop because of still running handler
					// cli, err := client.New(client.WithURL(getClientUrl(hasConfig)["UrlUxHttp"]))
					cli, err := client.New(client.WithPassword(cluster.ConfigData.Get().Secret()), client.WithURL(getClientUrl(hasConfig)["UrlInetHttp"]))
					require.NoError(t, err)
					daemonCli = daemoncmd.New(cli)
					e := daemonCli.Stop()
					require.NoErrorf(t, e, "unexpected error during stop: %s", e)
					require.NoFileExists(t, filepath.Join(rawconfig.Paths.Var, "osvcd.pid"))
				})
				require.False(t, t.Failed(), "can't continue test: initial daemonCli.Start() has errors")

				t.Run("Verify initial daemon start from daemonCli.Start() returns no error after daemonCli.Stop()", func(t *testing.T) {
					select {
					case err := <-startError:
						require.NoErrorf(t, err, "daemonCli.Start() returns unexpected error %s", err)
					case <-time.After(4 * time.Second):
						t.Fatalf("initial daemonCli.Start() should returns after daemonCli.Stop() succeeds")
					}
				})

				require.False(t, t.Failed(), "can't continue test: initial daemonCli.Start() has errors")

				for name, url := range getClientUrl(hasConfig) {
					t.Run("check stop again with client "+name, func(t *testing.T) {
						cli, err := newClient(url)
						require.Nil(t, err)
						require.NoError(t, daemoncmd.New(cli).Stop())
						isRunning, err = daemonCli.IsRunning()
						require.NoError(t, err)
						require.False(t, isRunning)
					})
				}
			})
		})
	}
}
